package zeroone

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLabelUsersPagesUntilAShortPage(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if !strings.Contains(body.Query, "label_user") {
			t.Errorf("query %q", body.Query)
		}
		n := rosterPageSize
		if body.Variables["offset"].(float64) > 0 {
			n = 3
		}
		rows := make([]string, n)
		for i := range rows {
			rows[i] = fmt.Sprintf(`{"id":%d,"labelId":41,"userId":%d}`, i+1, i+100)
		}
		fmt.Fprintf(w, `{"data":{"label_user":[%s]}}`, strings.Join(rows, ","))
	}))
	defer srv.Close()
	c, err := NewClient(srv.URL, "secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.LabelUsers(context.Background(), []int64{41})
	if err != nil || len(got) != rosterPageSize+3 || calls != 2 {
		t.Fatalf("rows %d calls %d err %v", len(got), calls, err)
	}
	if _, err := c.LabelUsers(context.Background(), nil); err == nil {
		t.Fatal("empty label list accepted")
	}
}
