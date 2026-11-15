package app

import "testing"

func TestParseCohorts(t *testing.T) {
	got, err := ParseCohorts("")
	if err != nil || len(got) != 2 || got[0].ID != 4 || got[0].Label != "01 Coding - Second Cohort" || got[1].ID != 1 {
		t.Fatalf("default = %+v, %v", got, err)
	}
	got, err = ParseCohorts(" 7 = Third Cohort ")
	if err != nil || len(got) != 1 || got[0].ID != 7 || got[0].Label != "Third Cohort" {
		t.Fatalf("custom = %+v, %v", got, err)
	}
	for _, bad := range []string{"4", "x=Label", "4=", "4=A,4=B", "0=A"} {
		if _, err := ParseCohorts(bad); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
}
