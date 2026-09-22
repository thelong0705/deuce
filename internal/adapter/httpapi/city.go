package httpapi

import (
	"net/http"

	"github.com/thelong0705/deuce/internal/domain/entity"
)

type cityResponse struct {
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

type cityListResponse struct {
	Cities []cityResponse `json:"cities"`
}

func (s *Server) listCities(w http.ResponseWriter, r *http.Request) {
	cities, err := s.cities.List(r.Context())
	if err != nil {
		writeAppError(w, err)
		return
	}

	out := make([]cityResponse, 0, len(cities))
	for _, c := range cities {
		out = append(out, newCityResponse(c))
	}

	writeJSON(w, http.StatusOK, cityListResponse{Cities: out})
}

func newCityResponse(c entity.City) cityResponse {
	return cityResponse{Name: c.Name, Timezone: c.Timezone}
}
