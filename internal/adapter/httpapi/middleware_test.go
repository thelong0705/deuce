package httpapi_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/adapter/httpapi"
	"github.com/thelong0705/deuce/internal/adapter/httpapi/mocks"
	"github.com/thelong0705/deuce/internal/domain/entity"
)

func authenticatedUser() *entity.User {
	return &entity.User{
		ID:          uuid.New(),
		Email:       "alice@example.com",
		DisplayName: "alice",
		PhoneNumber: "+84901234567",
		Role:        entity.RolePlayer,
		IsActive:    true,
	}
}

func TestRequireAuth(t *testing.T) {
	user := authenticatedUser()

	tests := []struct {
		name string
		// cookie is sent with the request when non-empty
		cookie     string
		setup      func(users *mocks.MockUserUsecase)
		wantStatus int
		wantCode   string
		check      func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:   "a live session reaches the handler",
			cookie: rawToken,
			setup: func(users *mocks.MockUserUsecase) {
				users.EXPECT().Authenticate(mock.Anything, rawToken).Return(user, nil).Once()
			},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var got map[string]any
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))

				require.Equal(t, user.ID.String(), got["id"])
				require.Equal(t, "alice@example.com", got["email"])
				require.Equal(t, "player", got["role"])

				require.NotContains(t, got, "password")
				require.NotContains(t, got, "password_hash")
			},
		},
		{
			name: "no cookie is 401",
			setup: func(users *mocks.MockUserUsecase) {
				users.EXPECT().Authenticate(mock.Anything, "").
					Return(nil, entity.ErrSessionInvalid).Once()
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "session_invalid",
		},
		{
			name:   "an unknown or expired session is 401",
			cookie: "stale-token",
			setup: func(users *mocks.MockUserUsecase) {
				users.EXPECT().Authenticate(mock.Anything, "stale-token").
					Return(nil, entity.ErrSessionInvalid).Once()
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "session_invalid",
		},
		{
			name:   "an unexpected error is 500 without leaking detail",
			cookie: rawToken,
			setup: func(users *mocks.MockUserUsecase) {
				users.EXPECT().Authenticate(mock.Anything, mock.Anything).
					Return(nil, errors.New("pq: connection to 10.0.0.5 refused")).Once()
			},
			wantStatus: http.StatusInternalServerError,
			check: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.NotContains(t, rec.Body.String(), "10.0.0.5")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := mocks.NewMockUserUsecase(t)
			tt.setup(users)

			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: sessionName, Value: tt.cookie})
			}

			rec := httptest.NewRecorder()
			httpapi.NewServer(users).Handler().ServeHTTP(rec, req)

			require.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantCode != "" {
				var got errorBody
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
				require.Equal(t, tt.wantCode, got.Code)
			}

			if tt.check != nil {
				tt.check(t, rec)
			}
		})
	}
}

// Unprotected routes must stay reachable without a session.
func TestUnprotectedRoutesSkipAuth(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"healthz", http.MethodGet, "/healthz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := mocks.NewMockUserUsecase(t)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			httpapi.NewServer(users).Handler().ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			users.AssertNotCalled(t, "Authenticate")
		})
	}
}
