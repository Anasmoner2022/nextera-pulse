package sync

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"nextera-pulse/internal/store"
	"nextera-pulse/internal/zeroone"
)

// ApplyLiveProgress writes current 01Edu project progress for students already in the roster.
// Rows for unknown users are skipped. Existing completion flags are updated in place.
func ApplyLiveProgress(ctx context.Context, db *store.DB, rows []zeroone.LiveProgress) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	type identity struct {
		studentID int64
		sourceID  int64
	}
	bySource := map[int64]identity{}
	byLogin := map[string]identity{}
	listed, err := db.QueryContext(ctx, `SELECT id, source_user_id, login FROM students`)
	if err != nil {
		return 0, err
	}
	for listed.Next() {
		var person identity
		var login string
		if err := listed.Scan(&person.studentID, &person.sourceID, &login); err != nil {
			listed.Close()
			return 0, err
		}
		bySource[person.sourceID] = person
		byLogin[strings.ToLower(strings.TrimSpace(login))] = person
	}
	if err := listed.Err(); err != nil {
		listed.Close()
		return 0, err
	}
	listed.Close()

	objectsByPath := map[string]int64{}
	objectsBySlug := map[string]int64{}
	objectRows, err := db.QueryContext(ctx, `SELECT source_object_id, path FROM learning_objects WHERE TRIM(path) <> ''`)
	if err != nil {
		return 0, err
	}
	for objectRows.Next() {
		var objectID int64
		var path string
		if err := objectRows.Scan(&objectID, &path); err != nil {
			objectRows.Close()
			return 0, err
		}
		objectsByPath[path] = objectID
		if slug := pathSlug(path); slug != "" {
			objectsBySlug[slug] = objectID
		}
	}
	if err := objectRows.Err(); err != nil {
		objectRows.Close()
		return 0, err
	}
	objectRows.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	objectQuery := db.Rebind(`INSERT INTO learning_objects (source_object_id, name, type, path, author_login)
		VALUES (?, ?, ?, ?, '')
		ON CONFLICT (source_object_id) DO UPDATE SET
			name = CASE WHEN excluded.name = '' THEN learning_objects.name ELSE excluded.name END,
			type = CASE WHEN excluded.type = '' THEN learning_objects.type ELSE excluded.type END,
			path = CASE WHEN excluded.path = '' THEN learning_objects.path ELSE excluded.path END`)
	progressQuery := db.Rebind(`INSERT INTO learning_progress (source_progress_id, student_id, source_user_id, event_id, object_id, path, is_done, grade, group_id, updated_at)
		VALUES (?, ?, ?, NULL, ?, ?, ?, ?, NULL, ?)
		ON CONFLICT (source_progress_id) DO UPDATE SET
			is_done = excluded.is_done,
			grade = excluded.grade,
			object_id = excluded.object_id,
			path = CASE WHEN excluded.path = '' THEN learning_progress.path ELSE excluded.path END,
			updated_at = excluded.updated_at`)
	updateByID := db.Rebind(`UPDATE learning_progress SET is_done = ?, grade = ?, updated_at = ?, object_id = CASE WHEN ? = 0 THEN object_id ELSE ? END WHERE source_progress_id = ?`)
	updateByPath := db.Rebind(`UPDATE learning_progress SET is_done = ?, grade = ?, updated_at = ? WHERE student_id = ? AND path = ?`)
	now := time.Now().UTC().Format(time.RFC3339)
	updated := 0
	for _, row := range rows {
		person, known := byLogin[strings.ToLower(strings.TrimSpace(row.Login()))]
		if !known {
			person, known = bySource[row.UserID]
		}
		objectID := row.ObjectID
		if row.Object != nil && row.Object.ID > 0 {
			objectID = row.Object.ID
			if _, err := tx.ExecContext(ctx, objectQuery, row.Object.ID, row.Object.Name, row.Object.Type, row.Object.Path); err != nil {
				return 0, err
			}
		}
		if objectID == 0 {
			objectID = objectsByPath[row.Path]
		}
		if objectID == 0 {
			objectID = objectsBySlug[pathSlug(row.Path)]
		}
		if !known || (objectID == 0 && row.ID <= 0) {
			continue
		}
		done := 0
		if row.IsDone {
			done = 1
		}
		if row.ID > 0 {
			result, err := tx.ExecContext(ctx, updateByID, done, row.Grade, now, objectID, objectID, row.ID)
			if err != nil {
				return 0, err
			}
			if changed, _ := result.RowsAffected(); changed > 0 {
				updated += int(changed)
				continue
			}
		}
		if objectID == 0 {
			continue
		}
		if row.ID <= 0 {
			result, err := tx.ExecContext(ctx, updateByPath, done, row.Grade, now, person.studentID, row.Path)
			if err != nil {
				return 0, err
			}
			changed, _ := result.RowsAffected()
			updated += int(changed)
			continue
		}
		if _, err := tx.ExecContext(ctx, progressQuery, row.ID, person.studentID, person.sourceID, objectID, row.Path, done, row.Grade, now); err != nil {
			return 0, err
		}
		updated++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return updated, nil
}

func pathSlug(path string) string {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" {
		return ""
	}
	if slash := strings.LastIndex(path, "/"); slash >= 0 {
		path = path[slash+1:]
	}
	return strings.ToLower(path)
}
