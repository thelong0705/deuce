package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

// sessionCookie is the cookie the session token travels in.
const sessionCookie = "deuce_session"

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type sessionResponse struct {
	UserID    uuid.UUID `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeBody(w, http.StatusBadRequest, invalidBodyResponse)
		return
	}

	token, session, err := s.users.Login(r.Context(), entity.LoginInput{
		Email:     req.Email,
		Password:  req.Password,
		UserAgent: r.UserAgent(),
		ClientIP:  clientIP(r),
	})
	if err != nil {
		writeAppError(w, err)
		return
	}

	s.setSessionCookie(w, token, session.ExpiresAt)

	writeJSON(w, http.StatusCreated, sessionResponse{
		UserID:    session.UserID,
		ExpiresAt: session.ExpiresAt,
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.users.Logout(r.Context(), sessionToken(r)); err != nil {
		writeAppError(w, err)
		return
	}

	s.clearSessionCookie(w)

	w.WriteHeader(http.StatusNoContent)
}

// sessionToken reads the session token from the cookie, returning "" when
// absent.
func sessionToken(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}

	return c.Value
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	// Secure has to match the cookie being cleared, or the browser treats this
	// as a different cookie and the old one survives.
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}

	return r.RemoteAddr
}

func (s *Server) currentUser(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeInternalError(w)
		return
	}

	writeJSON(w, http.StatusOK, newUserResponse(*user))
}
