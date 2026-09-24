package httpapi

import (
	"encoding/json"
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

// toEntityRole names the roles the wire accepts, so an unknown one is refused
// here rather than carried into the domain as a Role. Absent stays empty:
// the field is optional and Register defaults it.
func toEntityRole(s string) (entity.Role, error) {
	switch s {
	case "":
		return "", nil
	case "player":
		return entity.RolePlayer, nil
	case "owner":
		return entity.RoleOwner, nil
	default:
		return "", entity.ErrInvalidRole
	}
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeBody(w, http.StatusBadRequest, invalidBodyResponse)
		return
	}

	role, err := toEntityRole(req.Role)
	if err != nil {
		writeAppError(w, err)
		return
	}

	user, err := s.users.Register(r.Context(), entity.CreateUserInput{
		Email:       req.Email,
		Password:    req.Password,
		PhoneNumber: req.PhoneNumber,
		Role:        role,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, newUserResponse(*user))
}
