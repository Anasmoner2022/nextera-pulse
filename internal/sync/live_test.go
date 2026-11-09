package sync

import (
	"math"
	"nextera-pulse/internal/zeroone"
	"testing"
	"time"
)

func TestValidateEnrollments(t *testing.T) {
	u := zeroone.EventUser{ID: 1, UserID: 2, CreatedAt: time.Now(), Level: 5}
	cases := []struct {
		name  string
		users []zeroone.EventUser
		valid bool
	}{
		{"valid", []zeroone.EventUser{u}, true},
		{"empty", nil, false},
		{"duplicate", []zeroone.EventUser{u, u}, false},
		{"no enrollment timestamp", []zeroone.EventUser{{ID: 1, UserID: 2}}, false},
		{"non finite level", []zeroone.EventUser{{ID: 1, UserID: 2, CreatedAt: time.Now(), Level: math.Inf(1)}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateEnrollments(tc.users)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, err=%v", tc.valid, err)
			}
		})
	}
}
