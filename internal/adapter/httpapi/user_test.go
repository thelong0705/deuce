package httpapi_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/adapter/httpapi"
	"github.com/thelong0705/deuce/internal/adapter/httpapi/mocks"
	"github.com/thelong0705/deuce/internal/domain/entity"
)

const validBody = `{"email":"alice@example.com","password":"supersecret","phone_number":"+84901234567"}`

func registeredUser() *entity.User {
	return &entity.User{
		ID:          uuid.New(),
		Email:       "alice@example.com",
		DisplayName: "alice",
		PhoneNumber: "+84901234567",
		Role:        entity.RolePlayer,
		IsActive:    true,
		CreatedAt:   time.Now(),
	}
}

// do sends a request through the router and returns the recorded response.
func do(t *testing.T, users httpapi.UserRegister, venues httpapi.VenueCreator, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	httpapi.NewServer(users, venues).Handler().ServeHTTP(rec, req)

	return rec
}

func TestCreateUser(t *testing.T) {
	tests := []struct {
		name string
		body string
		// setup configures the mock; nil means a successful registration
		setup func(users *mocks.MockUserRegister)
		// wantNoCall asserts the use case was never reached
		wantNoCall bool
		wantStatus int
		wantErrMsg string
		check      func(t *testing.T, body []byte)
	}{
		{
			name:       "creates a user",
			body:       validBody,
			wantStatus: http.StatusCreated,
			check: func(t *testing.T, body []byte) {
				var got map[string]any
				require.NoError(t, json.Unmarshal(body, &got))

				require.Equal(t, "alice@example.com", got["email"])
				require.Equal(t, "alice", got["display_name"])
				require.Equal(t, "player", got["role"])
				require.NotEmpty(t, got["id"])

				// Nothing password-shaped may ever appear in a response.
				require.NotContains(t, got, "password")
				require.NotContains(t, got, "password_hash")
			},
		},
		{
			name: "passes the role through when given",
			body: `{"email":"ace@club.com","password":"supersecret","phone_number":"+84901234567","role":"owner"}`,
			setup: func(users *mocks.MockUserRegister) {
				users.EXPECT().
					Register(mock.Anything, mock.MatchedBy(func(in entity.CreateUserInput) bool {
						return in.Role == entity.RoleOwner
					})).
					Return(registeredUser(), nil).
					Once()
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "rejects malformed json",
			body:       `{"email":`,
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid JSON body",
		},
		{
			name:       "rejects an unknown field",
			body:       `{"email":"a@b.com","password":"supersecret","phone_number":"+84901234567","admin":true}`,
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid JSON body",
		},
		{
			name: "a validation error becomes 400",
			body: validBody,
			setup: func(users *mocks.MockUserRegister) {
				users.EXPECT().Register(mock.Anything, mock.Anything).
					Return(nil, entity.ErrPasswordTooShort).Once()
			},
			wantStatus: http.StatusBadRequest,
			wantErrMsg: entity.ErrPasswordTooShort.Error(),
		},
		{
			name: "a duplicate email becomes 409",
			body: validBody,
			setup: func(users *mocks.MockUserRegister) {
				users.EXPECT().Register(mock.Anything, mock.Anything).
					Return(nil, entity.ErrEmailTaken).Once()
			},
			wantStatus: http.StatusConflict,
			wantErrMsg: entity.ErrEmailTaken.Error(),
		},
		{
			name: "an unexpected error becomes 500 without leaking detail",
			body: validBody,
			setup: func(users *mocks.MockUserRegister) {
				users.EXPECT().Register(mock.Anything, mock.Anything).
					Return(nil, errors.New("pq: connection to 10.0.0.5 refused")).Once()
			},
			wantStatus: http.StatusInternalServerError,
			wantErrMsg: "could not create user",
			check: func(t *testing.T, body []byte) {
				require.NotContains(t, string(body), "10.0.0.5")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := mocks.NewMockUserRegister(t)

			switch {
			case tt.setup != nil:
				tt.setup(users)
			case !tt.wantNoCall:
				users.EXPECT().Register(mock.Anything, mock.Anything).
					Return(registeredUser(), nil).Once()
			}

			rec := do(t, users, mocks.NewMockVenueCreator(t), http.MethodPost, "/users", tt.body)

			require.Equal(t, tt.wantStatus, rec.Code)
			require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			if tt.wantNoCall {
				users.AssertNotCalled(t, "Register")
			}

			if tt.wantErrMsg != "" {
				var got errorBody
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
				require.Equal(t, tt.wantErrMsg, got.Error)
			}

			if tt.check != nil {
				tt.check(t, rec.Body.Bytes())
			}
		})
	}
}

type errorBody struct {
	Error string `json:"error"`
}

func TestHealthz(t *testing.T) {
	rec := do(t, mocks.NewMockUserRegister(t), mocks.NewMockVenueCreator(t), http.MethodGet, "/healthz", "")

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

func TestUnknownRouteIs404(t *testing.T) {
	rec := do(t, mocks.NewMockUserRegister(t), mocks.NewMockVenueCreator(t), http.MethodGet, "/nope", "")

	require.Equal(t, http.StatusNotFound, rec.Code)
}
