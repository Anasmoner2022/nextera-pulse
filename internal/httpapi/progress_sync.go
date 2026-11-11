package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"nextera-pulse/internal/store"
	"nextera-pulse/internal/sync"
	"nextera-pulse/internal/zeroone"
)

func (s *Server) rememberProgressSession(token string) {
	role, userID, expires, ok := zeroone.SessionAccess(token)
	if !ok {
		return
	}
	if !expires.IsZero() && time.Now().After(expires) {
		return
	}
	s.progress.mu.Lock()
	defer s.progress.mu.Unlock()
	if s.progress.token != "" && zeroone.CohortProgressRole(s.progress.role) && !zeroone.CohortProgressRole(role) {
		if s.progress.expires.IsZero() || time.Now().Before(s.progress.expires) {
			return
		}
	}
	s.progress.generation++
	s.progress.token = token
	s.progress.role = role
	s.progress.userID = userID
	s.progress.expires = expires
	s.progress.refreshed = time.Time{}
	s.progress.cohortSynced = time.Time{}
}

func (s *Server) liveProgressBoard(ctx context.Context, cohortID, eventID int, studentID int64) (store.ProgressBoard, error) {
	refreshed, err := s.refreshProjectProgress(ctx)
	if err != nil {
		log.Printf("project progress refresh failed: %v", err)
	}
	board, err := s.db.ProgressBoard(ctx, cohortID, eventID, studentID)
	if err != nil {
		return board, err
	}
	if !refreshed.IsZero() {
		board.SyncedAt = refreshed.UTC().Format(time.RFC3339)
	}
	return board, nil
}

func (s *Server) refreshProjectProgress(ctx context.Context) (time.Time, error) {
	s.progress.mu.Lock()
	if s.progress.token == "" {
		s.progress.mu.Unlock()
		return time.Time{}, nil
	}
	if !s.progress.expires.IsZero() && time.Now().After(s.progress.expires) {
		synced := s.progress.cohortSynced
		s.progress.mu.Unlock()
		return synced, nil
	}
	if !s.progress.refreshed.IsZero() && time.Since(s.progress.refreshed) < 2*time.Minute {
		synced := s.progress.cohortSynced
		s.progress.mu.Unlock()
		return synced, nil
	}
	if s.progress.inflight != nil {
		wait := s.progress.inflight
		s.progress.mu.Unlock()
		select {
		case <-wait:
		case <-ctx.Done():
			return time.Time{}, ctx.Err()
		}
		s.progress.mu.Lock()
		synced := s.progress.cohortSynced
		s.progress.mu.Unlock()
		return synced, nil
	}
	token, role := s.progress.token, s.progress.role
	userID := s.progress.userID
	generation := s.progress.generation
	done := make(chan struct{})
	s.progress.inflight = done
	s.progress.mu.Unlock()

	applied, cohortWide, err := s.pullProjectProgress(ctx, token, role, userID)

	s.progress.mu.Lock()
	if err == nil && s.progress.generation == generation {
		s.progress.refreshed = time.Now().UTC()
		if cohortWide {
			log.Printf("project progress refresh updated %d rows", applied)
			s.progress.cohortSynced = s.progress.refreshed
		} else {
			log.Printf("project progress updated %d personal rows; the cohort board stays on the archive", applied)
			s.progress.cohortSynced = time.Time{}
		}
	}
	synced := time.Time{}
	if err == nil && cohortWide && s.progress.generation == generation {
		synced = s.progress.cohortSynced
	} else if err == nil && s.progress.generation != generation {
		synced = s.progress.cohortSynced
	}
	close(done)
	s.progress.inflight = nil
	s.progress.mu.Unlock()
	if err != nil {
		return time.Time{}, err
	}
	return synced, nil
}

func (s *Server) pullProjectProgress(ctx context.Context, token, role string, userID int64) (int, bool, error) {
	var sourceRole string
	err := s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT source_role FROM students WHERE source_user_id = ?`), userID).Scan(&sourceRole)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("project progress could not read the signed-in role: %v", err)
	}
	cohort := zeroone.CohortProgressRole(role) || sourceRole == "admin campus" || sourceRole == "admin"
	logins, err := s.progressLogins(ctx, cohort, userID)
	if err != nil || len(logins) == 0 {
		return 0, false, err
	}
	paths, err := s.projectPaths(ctx)
	if err != nil || len(paths) == 0 {
		return 0, false, err
	}
	allowed := zeroone.AllowedRoles(token)
	if len(allowed) > 0 {
		log.Printf("project progress token allows: %s", strings.Join(allowed, ", "))
	}
	// Empty role uses the token's default Hasura role. Staff roles come next,
	// including the underscore names 01Edu puts on an admin read token.
	roles := append([]string{""}, allowed...)
	if cohort {
		roles = append(roles, "admin_read_only", "campus-admin", "campus_admin", "campus-admin-read-only", "campus_admin_read_only", "admin", "admin-read-only")
	} else if role != "" {
		roles = append(roles, role)
	}
	endpoint := strings.TrimRight(zeroone.SchoolBaseURL(), "/") + "/api/graphql-engine/v1/graphql"
	client, err := zeroone.NewClient(endpoint, token, &http.Client{Timeout: 60 * time.Second})
	if err != nil {
		return 0, false, err
	}
	var best []zeroone.LiveProgress
	var bestRole string
	var lastErr error
	seen := map[string]bool{}
	for _, tryRole := range roles {
		label := tryRole
		if label == "" {
			label = "default"
		}
		if seen[label] {
			continue
		}
		seen[label] = true
		client.SetRole(tryRole)
		live, err := client.CohortProgress(ctx, logins, paths)
		if err != nil {
			lastErr = err
			if issue := client.SchemaIssue(); issue != "" {
				log.Printf("project progress role %s rejected: %s", label, issue)
			}
			continue
		}
		log.Printf("project progress role %s returned %d rows for %d logins", label, len(live), distinctProgressLogins(live))
		if len(live) > len(best) {
			best = live
			bestRole = label
		}
	}
	if len(best) == 0 {
		return 0, false, lastErr
	}
	log.Printf("project progress using role %s", bestRole)
	applied, err := sync.ApplyLiveProgress(ctx, s.db, best)
	if err != nil {
		return 0, false, err
	}
	return applied, distinctProgressLogins(best) > 1 || zeroone.CohortProgressRole(bestRole), nil
}

func distinctProgressLogins(rows []zeroone.LiveProgress) int {
	seen := map[string]struct{}{}
	for _, row := range rows {
		login := strings.ToLower(strings.TrimSpace(row.Login()))
		if login == "" {
			continue
		}
		seen[login] = struct{}{}
	}
	return len(seen)
}

func (s *Server) progressLogins(ctx context.Context, cohort bool, userID int64) ([]string, error) {
	query := `SELECT login FROM students WHERE TRIM(login) <> ''`
	args := []any{}
	if !cohort {
		query = `SELECT login FROM students WHERE source_user_id = ?`
		args = append(args, userID)
	}
	rows, err := s.db.QueryContext(ctx, s.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logins []string
	for rows.Next() {
		var login string
		if err := rows.Scan(&login); err != nil {
			return nil, err
		}
		logins = append(logins, login)
	}
	return logins, rows.Err()
}

func (s *Server) projectPaths(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path FROM learning_objects WHERE lower(type) = 'project' AND TRIM(path) <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	var paths []string
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		add(path)
		slug := strings.Trim(path, "/")
		if slash := strings.LastIndex(slug, "/"); slash >= 0 {
			slug = slug[slash+1:]
		}
		add("/cairo/module/" + slug)
		add("root/01-edu_module/" + slug + "/")
	}
	return paths, rows.Err()
}
