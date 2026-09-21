package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

type createUserRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	PhoneNumber string `json:"phone_number"`
	Role        string `json:"role,omitempty"`
}

type userResponse struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	PhoneNumber string    `json:"phone_number"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

func newUserResponse(u entity.User) userResponse {
	return userResponse{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		PhoneNumber: u.PhoneNumber,
		Role:        u.Role.String(),
		CreatedAt:   u.CreatedAt,
	}
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	user, err := s.users.Register(r.Context(), entity.CreateUserInput{
		Email:       req.Email,
		Password:    req.Password,
		PhoneNumber: req.PhoneNumber,
		Role:        entity.Role(req.Role),
	})
	if err != nil {
		writeRegisterError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, newUserResponse(*user))
}

func writeRegisterError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, entity.ErrEmailTaken):
		writeError(w, http.StatusConflict, err.Error())

	case errors.Is(err, entity.ErrInvalidEmail),
		errors.Is(err, entity.ErrPasswordTooShort),
		errors.Is(err, entity.ErrPasswordTooLong),
		errors.Is(err, entity.ErrPhoneRequired),
		errors.Is(err, entity.ErrInvalidRole):
		writeError(w, http.StatusBadRequest, err.Error())

	default:
		writeError(w, http.StatusInternalServerError, "could not create user")
	}
}
