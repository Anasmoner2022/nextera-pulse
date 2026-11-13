package httpapi

import (
	"net/http"
	"testing"
)

func TestSyncStatusWithoutCredentialsIsOffline(t *testing.T) {
	api := newTestAPI(t)
	rec := api.do("GET", "/api/v1/sync/status", "", api.cookieFor(managerID))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if body := decode[struct{ Mode string }](t, rec); body.Mode != "offline" {
		t.Fatalf("mode %q", body.Mode)
	}
}

func TestCohortStudentsReportTheirDataSource(t *testing.T) {
	api := newTestAPI(t)
	rec := api.do("GET", "/api/v1/cohorts/97/students", "", api.cookieFor(managerID))
	body := decode[struct {
		DataSource string `json:"dataSource"`
	}](t, rec)
	if rec.Code != http.StatusOK || body.DataSource != "offline" {
		t.Fatalf("status %d source %q", rec.Code, body.DataSource)
	}
}
