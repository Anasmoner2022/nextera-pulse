package app

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestNoHardcodedEventID fails when application code writes the curriculum
// event id literally. The default lives only in internal/sync/program.go
// (pulsesync.DefaultEventID); everything else reads ZEROONE_EVENT_ID through
// the configuration. Tests, fixtures, docs and .env.example are exempt.
func TestNoHardcodedEventID(t *testing.T) {
	root := filepath.Join("..", "..")
	literal := regexp.MustCompile(`\b97\b`)
	allowed := map[string]bool{filepath.Join("internal", "sync", "program.go"): true}
	var offenders []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "dist", "docs":
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".ts" && ext != ".tsx" && ext != ".sql" && ext != ".mjs" {
			return nil
		}
		if allowed[rel] || strings.HasSuffix(path, "_test.go") || strings.Contains(path, ".test.") ||
			strings.HasSuffix(path, "mockStudents.ts") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			if literal.MatchString(line) {
				offenders = append(offenders, rel+":"+itoa(i+1)+": "+strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("hard-coded event id; use the configured event (ZEROONE_EVENT_ID):\n%s", strings.Join(offenders, "\n"))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
