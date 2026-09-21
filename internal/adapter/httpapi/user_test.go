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
	"github.com/thelong0705/deuce/internal/pkg/apperr"
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

// deps are the use cases a test wires into the server. Any left nil become a
// mock with no expectations, so a handler that reaches for one fails the test.
type deps struct {
	users  httpapi.UserUsecase
	venues httpapi.VenueUsecase
	courts httpapi.CourtUsecase
}

func (d deps) handler(t *testing.T) http.Handler {
	t.Helper()

	if d.users == nil {
		d.users = mocks.NewMockUserUsecase(t)
	}
	if d.venues == nil {
		d.venues = mocks.NewMockVenueUsecase(t)
	}
	if d.courts == nil {
		d.courts = mocks.NewMockCourtUsecase(t)
	}

	return httpapi.NewServer(d.users, d.venues, d.courts).Handler()
}

// do sends a request through the router and returns the recorded response.
func do(t *testing.T, d deps, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	return send(t, d, newRequest(method, path, body))
}

// doAuthed is do with a session cookie attached.
func doAuthed(t *testing.T, d deps, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := newRequest(method, path, body)
	req.AddCookie(&http.Cookie{Name: sessionName, Value: rawToken})

	return send(t, d, req)
}

func newRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	return req
}

func send(t *testing.T, d deps, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	d.handler(t).ServeHTTP(rec, req)

	return rec
}

func TestCreateUser(t *testing.T) {
	tests := []struct {
		name string
		body string
		// setup configures the mock; nil means a successful registration
		setup func(users *mocks.MockUserUsecase)
		// wantNoCall asserts the use case was never reached
		wantNoCall bool
		wantStatus int
		wantErrMsg string
		wantCode   string
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
			setup: func(users *mocks.MockUserUsecase) {
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
			wantCode:   "invalid_body",
		},
		{
			name:       "rejects an unknown field",
			body:       `{"email":"a@b.com","password":"supersecret","phone_number":"+84901234567","admin":true}`,
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid JSON body",
			wantCode:   "invalid_body",
		},
		{
			name: "a validation error becomes 400",
			body: validBody,
			setup: func(users *mocks.MockUserUsecase) {
				users.EXPECT().Register(mock.Anything, mock.Anything).
					Return(nil, entity.ErrPasswordTooShort).Once()
			},
			wantStatus: http.StatusBadRequest,
			wantErrMsg: entity.ErrPasswordTooShort.Error(),
			wantCode:   "password_too_short",
		},
		{
			name: "a duplicate email becomes 409",
			body: validBody,
			setup: func(users *mocks.MockUserUsecase) {
				users.EXPECT().Register(mock.Anything, mock.Anything).
					Return(nil, entity.ErrEmailTaken).Once()
			},
			wantStatus: http.StatusConflict,
			wantErrMsg: entity.ErrEmailTaken.Error(),
			wantCode:   "email_taken",
		},
		{
			// The handler has never heard of this error. It maps correctly
			// because the kind travels with it.
			name: "an unknown domain error maps by kind",
			body: validBody,
			setup: func(users *mocks.MockUserUsecase) {
				users.EXPECT().Register(mock.Anything, mock.Anything).
					Return(nil, apperr.New(apperr.KindForbidden, "not_allowed", "not allowed here")).Once()
			},
			wantStatus: http.StatusForbidden,
			wantErrMsg: "not allowed here",
			wantCode:   "not_allowed",
		},
		{
			name: "a not-found kind becomes 404",
			body: validBody,
			setup: func(users *mocks.MockUserUsecase) {
				users.EXPECT().Register(mock.Anything, mock.Anything).
					Return(nil, apperr.New(apperr.KindNotFound, "user_not_found", "user not found")).Once()
			},
			wantStatus: http.StatusNotFound,
			wantErrMsg: "user not found",
			wantCode:   "user_not_found",
		},
		{
			name: "an unexpected error becomes 500 without leaking detail",
			body: validBody,
			setup: func(users *mocks.MockUserUsecase) {
				users.EXPECT().Register(mock.Anything, mock.Anything).
					Return(nil, errors.New("pq: connection to 10.0.0.5 refused")).Once()
			},
			wantStatus: http.StatusInternalServerError,
			wantErrMsg: "internal error",
			wantCode:   "internal_error",
			check: func(t *testing.T, body []byte) {
				require.NotContains(t, string(body), "10.0.0.5")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := mocks.NewMockUserUsecase(t)

			switch {
			case tt.setup != nil:
				tt.setup(users)
			case !tt.wantNoCall:
				users.EXPECT().Register(mock.Anything, mock.Anything).
					Return(registeredUser(), nil).Once()
			}

			rec := do(t, deps{users: users}, http.MethodPost, "/users", tt.body)

			require.Equal(t, tt.wantStatus, rec.Code)
			require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			if tt.wantNoCall {
				users.AssertNotCalled(t, "Register")
			}

			if tt.wantErrMsg != "" {
				var got errorBody
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
				require.Equal(t, tt.wantErrMsg, got.Error)
				require.Equal(t, tt.wantCode, got.Code)
			}

			if tt.check != nil {
				tt.check(t, rec.Body.Bytes())
			}
		})
	}
}

type errorBody struct {
	Code  string `json:"code"`
	Error string `json:"error"`
}

func TestHealthz(t *testing.T) {
	rec := do(t, deps{}, http.MethodGet, "/healthz", "")

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

func TestUnknownRouteIs404(t *testing.T) {
	rec := do(t, deps{}, http.MethodGet, "/nope", "")

	require.Equal(t, http.StatusNotFound, rec.Code)
}
