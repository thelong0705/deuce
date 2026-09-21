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

const (
	loginBody   = `{"email":"alice@example.com","password":"supersecret"}`
	sessionName = "deuce_session"
	rawToken    = "a-raw-session-token"
)

var loginUserID = uuid.New()

func issuedSession() *entity.Session {
	return &entity.Session{
		UserID:    loginUserID,
		ExpiresAt: time.Now().Add(time.Hour),
	}
}

func cookieNamed(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()

	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}

	return nil
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name string
		body string
		// setup configures the mock; nil means a successful login
		setup func(login *mocks.MockUserUsecase)
		// wantNoCall asserts the use case was never reached
		wantNoCall bool
		wantStatus int
		wantCode   string
		check      func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:       "starts a session and sets the cookie",
			body:       loginBody,
			wantStatus: http.StatusCreated,
			check: func(t *testing.T, rec *httptest.ResponseRecorder) {
				c := cookieNamed(t, rec, sessionName)
				require.NotNil(t, c)
				require.Equal(t, rawToken, c.Value)
				require.True(t, c.HttpOnly, "javascript must not be able to read the session")
				require.True(t, c.Secure)
				require.Equal(t, http.SameSiteLaxMode, c.SameSite)
				require.Equal(t, "/", c.Path)

				var got map[string]any
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
				require.Equal(t, loginUserID.String(), got["user_id"])
				require.NotEmpty(t, got["expires_at"])

				// The body must not repeat the token; the cookie is the only copy.
				require.NotContains(t, rec.Body.String(), rawToken)
			},
		},
		{
			name:       "rejects malformed json",
			body:       `{"email":`,
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_body",
		},
		{
			name:       "rejects an unknown field",
			body:       `{"email":"a@b.com","password":"x","admin":true}`,
			wantNoCall: true,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_body",
		},
		{
			name: "bad credentials are 401 and set no cookie",
			body: loginBody,
			setup: func(login *mocks.MockUserUsecase) {
				login.EXPECT().Login(mock.Anything, mock.Anything).
					Return("", nil, entity.ErrInvalidCredentials).Once()
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "invalid_credentials",
			check: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Nil(t, cookieNamed(t, rec, sessionName))
			},
		},
		{
			name: "a malformed email is 400",
			body: `{"email":"nope","password":"supersecret"}`,
			setup: func(login *mocks.MockUserUsecase) {
				login.EXPECT().Login(mock.Anything, mock.Anything).
					Return("", nil, entity.ErrInvalidEmail).Once()
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_email",
		},
		{
			name: "an unexpected error is 500 without leaking detail",
			body: loginBody,
			setup: func(login *mocks.MockUserUsecase) {
				login.EXPECT().Login(mock.Anything, mock.Anything).
					Return("", nil, errors.New("pq: connection to 10.0.0.5 refused")).Once()
			},
			wantStatus: http.StatusInternalServerError,
			check: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.NotContains(t, rec.Body.String(), "10.0.0.5")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			login := mocks.NewMockUserUsecase(t)

			switch {
			case tt.setup != nil:
				tt.setup(login)
			case !tt.wantNoCall:
				login.EXPECT().Login(mock.Anything, mock.Anything).
					Return(rawToken, issuedSession(), nil).Once()
			}

			req := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			rec := httptest.NewRecorder()
			httpapi.NewServer(login, mocks.NewMockVenueUsecase(t)).Handler().ServeHTTP(rec, req)

			require.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantNoCall {
				login.AssertNotCalled(t, "Login")
			}

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

func TestLoginPassesTheRequestContext(t *testing.T) {
	login := mocks.NewMockUserUsecase(t)

	login.EXPECT().
		Login(mock.Anything, mock.MatchedBy(func(in entity.LoginInput) bool {
			return in.Email == "alice@example.com" &&
				in.UserAgent == "curl/8" &&
				in.ClientIP == "203.0.113.9"
		})).
		Return(rawToken, issuedSession(), nil).
		Once()

	req := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(loginBody))
	req.Header.Set("User-Agent", "curl/8")
	req.Header.Set("X-Forwarded-For", "203.0.113.9")

	rec := httptest.NewRecorder()
	httpapi.NewServer(login, mocks.NewMockVenueUsecase(t)).Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestLogout(t *testing.T) {
	tests := []struct {
		name string
		// cookie is sent with the request when non-empty
		cookie     string
		setup      func(logout *mocks.MockUserUsecase)
		wantStatus int
		check      func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:   "ends the session and clears the cookie",
			cookie: rawToken,
			setup: func(logout *mocks.MockUserUsecase) {
				logout.EXPECT().Logout(mock.Anything, rawToken).Return(nil).Once()
			},
			wantStatus: http.StatusNoContent,
			check: func(t *testing.T, rec *httptest.ResponseRecorder) {
				c := cookieNamed(t, rec, sessionName)
				require.NotNil(t, c)
				require.Empty(t, c.Value)
				require.Less(t, c.MaxAge, 0, "the cookie must be expired, not just emptied")
			},
		},
		{
			name: "without a cookie it is still 204",
			setup: func(logout *mocks.MockUserUsecase) {
				logout.EXPECT().Logout(mock.Anything, "").Return(nil).Once()
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:   "a storage failure is 500",
			cookie: rawToken,
			setup: func(logout *mocks.MockUserUsecase) {
				logout.EXPECT().Logout(mock.Anything, mock.Anything).
					Return(errors.New("pq: connection refused")).Once()
			},
			wantStatus: http.StatusInternalServerError,
			check: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Nil(t, cookieNamed(t, rec, sessionName), "a failed logout must not clear the cookie")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logout := mocks.NewMockUserUsecase(t)
			tt.setup(logout)

			req := httptest.NewRequest(http.MethodDelete, "/sessions", nil)
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: sessionName, Value: tt.cookie})
			}

			rec := httptest.NewRecorder()
			httpapi.NewServer(logout, mocks.NewMockVenueUsecase(t)).Handler().ServeHTTP(rec, req)

			require.Equal(t, tt.wantStatus, rec.Code)

			if tt.check != nil {
				tt.check(t, rec)
			}
		})
	}
}
