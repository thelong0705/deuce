package entity

import "github.com/thelong0705/deuce/internal/pkg/apperr"

// City is somewhere deuce operates. Cities arrive by migration, not by anyone
// typing one in, and each decides the timezone its venues keep hours in.
type City struct {
	Name     string
	Timezone string
}

var ErrVenueCityUnsupported = apperr.New(
	apperr.KindInvalid,
	"venue_city_unsupported",
	"deuce does not operate in that city yet",
)
