package sync

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"log"
	"strconv"
	"strings"

	"nextera-pulse/internal/store"
)

func ingestLearningArchive(ctx context.Context, db *store.DB, files map[string]*zip.File, students map[int64]int64) error {
	if err := ingestObjects(ctx, db, files["csv/object.csv"]); err != nil {
		return err
	}
	if err := ingestProgress(ctx, db, files["csv/progress.csv"], students); err != nil {
		return err
	}
	if err := ingestResults(ctx, db, files["csv/result.csv"], students); err != nil {
		return err
	}
	if err := ingestGroups(ctx, db, files["csv/group.csv"]); err != nil {
		return err
	}
	if err := ingestAudits(ctx, db, files["csv/audit.csv"]); err != nil {
		return err
	}
	if err := ingestTransactions(ctx, db, files["csv/transaction.csv"], students); err != nil {
		return err
	}
	if err := ingestLabels(ctx, db, files["csv/label.csv"]); err != nil {
		return err
	}
	if err := ingestLabelUsers(ctx, db, files["csv/label_user.csv"], students); err != nil {
		return err
	}
	return ingestUserRoles(ctx, db, files["csv/user.csv"])
}

func ingestObjects(ctx context.Context, db *store.DB, file *zip.File) error {
	if file == nil {
		return nil
	}
	query := db.Rebind(`INSERT INTO learning_objects (source_object_id, name, type, path, author_login)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (source_object_id) DO UPDATE SET
			name = excluded.name, type = excluded.type, path = excluded.path, author_login = excluded.author_login`)
	count, err := streamCSV(ctx, db, file, query, func(idx map[string]int, row []string) []any {
		id := csvInt(row, idx, "id")
		if id == 0 {
			return nil
		}
		path := csvValue(row, idx, "referencepath")
		return []any{id, csvValue(row, idx, "name"), csvValue(row, idx, "type"), path, csvValue(row, idx, "authorlogin")}
	})
	log.Printf("ingested %d learning objects", count)
	return err
}

func ingestProgress(ctx context.Context, db *store.DB, file *zip.File, students map[int64]int64) error {
	if file == nil {
		return nil
	}
	query := db.Rebind(`INSERT INTO learning_progress (source_progress_id, student_id, source_user_id, event_id, object_id, path, is_done, grade, group_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (source_progress_id) DO UPDATE SET
			is_done = excluded.is_done, grade = excluded.grade, path = excluded.path, updated_at = excluded.updated_at`)
	count, err := streamCSV(ctx, db, file, query, func(idx map[string]int, row []string) []any {
		userID := int64(csvInt(row, idx, "userid"))
		studentID, ok := students[userID]
		id := csvInt(row, idx, "id")
		if !ok || id == 0 {
			return nil
		}
		return []any{id, studentID, userID, csvInt(row, idx, "eventid"), csvInt(row, idx, "objectid"), csvValue(row, idx, "path"), csvBool(row, idx, "isdone"), csvFloat(row, idx, "grade"), csvInt(row, idx, "groupid"), csvValue(row, idx, "updatedat")}
	})
	log.Printf("ingested %d progress rows", count)
	return err
}

func ingestResults(ctx context.Context, db *store.DB, file *zip.File, students map[int64]int64) error {
	if file == nil {
		return nil
	}
	query := db.Rebind(`INSERT INTO learning_results (source_result_id, student_id, source_user_id, object_id, path, grade, type, is_last, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (source_result_id) DO UPDATE SET grade = excluded.grade, is_last = excluded.is_last`)
	count, err := streamCSV(ctx, db, file, query, func(idx map[string]int, row []string) []any {
		userID := int64(csvInt(row, idx, "userid"))
		studentID, ok := students[userID]
		id := csvInt(row, idx, "id")
		if !ok || id == 0 {
			return nil
		}
		return []any{id, studentID, userID, csvInt(row, idx, "objectid"), csvValue(row, idx, "path"), csvFloat(row, idx, "grade"), csvValue(row, idx, "type"), csvBool(row, idx, "islast"), csvValue(row, idx, "createdat")}
	})
	log.Printf("ingested %d results", count)
	return err
}

func ingestGroups(ctx context.Context, db *store.DB, file *zip.File) error {
	if file == nil {
		return nil
	}
	query := db.Rebind(`INSERT INTO learning_groups (source_group_id, event_id, object_id, path, status, captain_login)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (source_group_id) DO UPDATE SET status = excluded.status, captain_login = excluded.captain_login`)
	count, err := streamCSV(ctx, db, file, query, func(idx map[string]int, row []string) []any {
		id := csvInt(row, idx, "id")
		if id == 0 {
			return nil
		}
		return []any{id, csvInt(row, idx, "eventid"), csvInt(row, idx, "objectid"), csvValue(row, idx, "path"), csvValue(row, idx, "status"), csvValue(row, idx, "captainlogin")}
	})
	log.Printf("ingested %d groups", count)
	return err
}

func ingestAudits(ctx context.Context, db *store.DB, file *zip.File) error {
	if file == nil {
		return nil
	}
	query := db.Rebind(`INSERT INTO learning_audits (source_audit_id, group_id, auditor_login, grade, closure_type, audited_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (source_audit_id) DO UPDATE SET grade = excluded.grade, closure_type = excluded.closure_type`)
	count, err := streamCSV(ctx, db, file, query, func(idx map[string]int, row []string) []any {
		id := csvInt(row, idx, "id")
		if id == 0 {
			return nil
		}
		return []any{id, csvInt(row, idx, "groupid"), csvValue(row, idx, "auditorlogin"), csvFloat(row, idx, "grade"), csvValue(row, idx, "closuretype"), csvValue(row, idx, "auditedat")}
	})
	log.Printf("ingested %d audits", count)
	return err
}

func ingestTransactions(ctx context.Context, db *store.DB, file *zip.File, students map[int64]int64) error {
	if file == nil {
		return nil
	}
	query := db.Rebind(`INSERT INTO learning_transactions (source_transaction_id, student_id, source_user_id, object_id, path, type, amount, is_bonus, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (source_transaction_id) DO UPDATE SET amount = excluded.amount, type = excluded.type`)
	count, err := streamCSV(ctx, db, file, query, func(idx map[string]int, row []string) []any {
		userID := int64(csvInt(row, idx, "userid"))
		studentID, ok := students[userID]
		id := csvInt(row, idx, "id")
		if !ok || id == 0 {
			return nil
		}
		return []any{id, studentID, userID, csvInt(row, idx, "objectid"), csvValue(row, idx, "path"), csvValue(row, idx, "type"), csvFloat(row, idx, "amount"), csvBool(row, idx, "isbonus"), csvValue(row, idx, "createdat")}
	})
	log.Printf("ingested %d transactions", count)
	return err
}

func ingestLabels(ctx context.Context, db *store.DB, file *zip.File) error {
	if file == nil {
		return nil
	}
	query := db.Rebind(`INSERT INTO cohort_labels (source_label_id, event_id, name, description)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (source_label_id) DO UPDATE SET name = excluded.name, description = excluded.description`)
	_, err := streamCSV(ctx, db, file, query, func(idx map[string]int, row []string) []any {
		id := csvInt(row, idx, "id")
		if id == 0 {
			return nil
		}
		return []any{id, csvInt(row, idx, "eventid"), csvValue(row, idx, "name"), csvValue(row, idx, "description")}
	})
	return err
}

func ingestUserRoles(ctx context.Context, db *store.DB, file *zip.File) error {
	if file == nil {
		return nil
	}
	query := db.Rebind(`UPDATE students SET source_role = ? WHERE source_user_id = ?`)
	count, err := streamCSV(ctx, db, file, query, func(idx map[string]int, row []string) []any {
		userID := csvInt(row, idx, "id")
		role := csvValue(row, idx, "role")
		if userID == 0 || role == "" {
			return nil
		}
		return []any{role, userID}
	})
	if err != nil {
		return err
	}
	log.Printf("stored 01Edu roles for %d users", count)
	return nil
}

func ingestLabelUsers(ctx context.Context, db *store.DB, file *zip.File, students map[int64]int64) error {
	if file == nil {
		return nil
	}
	query := db.Rebind(`INSERT INTO cohort_label_users (source_id, label_id, source_user_id, label_name)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (source_id) DO UPDATE SET label_id = excluded.label_id, label_name = excluded.label_name`)
	count, err := streamCSV(ctx, db, file, query, func(idx map[string]int, row []string) []any {
		userID := int64(csvInt(row, idx, "userid"))
		if _, ok := students[userID]; !ok {
			return nil
		}
		id := csvInt(row, idx, "id")
		if id == 0 {
			return nil
		}
		return []any{id, csvInt(row, idx, "labelid"), userID, csvValue(row, idx, "labelname")}
	})
	log.Printf("ingested %d cohort label memberships", count)
	return err
}

func streamCSV(ctx context.Context, db *store.DB, file *zip.File, query string, values func(map[string]int, []string) []any) (int, error) {
	rc, err := file.Open()
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	content, err := io.ReadAll(rc)
	if err != nil {
		return 0, err
	}
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))))
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = true
	header, err := reader.Read()
	if err != nil {
		return 0, err
	}
	idx := map[string]int{}
	for i, name := range header {
		idx[strings.Trim(strings.ToLower(name), "\ufeff \"")] = i
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	count := 0
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, err
		}
		args := values(idx, row)
		if args == nil {
			continue
		}
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return count, err
		}
		count++
		if count%5000 == 0 {
			if err := tx.Commit(); err != nil {
				return count, err
			}
			next, err := db.BeginTx(ctx, nil)
			if err != nil {
				committed = true
				return count, err
			}
			tx = next
		}
	}
	if err := tx.Commit(); err != nil {
		return count, err
	}
	committed = true
	return count, nil
}

func csvValue(row []string, idx map[string]int, name string) string {
	i, ok := idx[name]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func csvInt(row []string, idx map[string]int, name string) int {
	n, _ := strconv.Atoi(csvValue(row, idx, name))
	return n
}

func csvFloat(row []string, idx map[string]int, name string) any {
	text := csvValue(row, idx, name)
	if text == "" {
		return nil
	}
	n, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil
	}
	return n
}

func csvBool(row []string, idx map[string]int, name string) int {
	switch strings.ToLower(csvValue(row, idx, name)) {
	case "true", "t", "1", "yes":
		return 1
	default:
		return 0
	}
}
