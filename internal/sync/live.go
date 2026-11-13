package sync

import (
	"errors"
	"math"

	"nextera-pulse/internal/zeroone"
)

// ValidateEnrollments rejects an event_user response that is empty, has
// duplicate users, or carries impossible values, so a bad response never
// overwrites good data.
func ValidateEnrollments(users []zeroone.EventUser) error {
	if len(users) == 0 {
		return errors.New("empty 01Edu enrollment response; refusing sync")
	}
	seen := make(map[int64]bool, len(users))
	for _, u := range users {
		if u.ID <= 0 || u.UserID <= 0 || u.CreatedAt.IsZero() || math.IsInf(u.Level, 0) || math.IsNaN(u.Level) || u.Level < 0 {
			return errors.New("invalid 01Edu enrollment data; refusing sync")
		}
		if seen[u.UserID] {
			return errors.New("duplicate source user in enrollment response; refusing sync")
		}
		seen[u.UserID] = true
	}
	return nil
}
