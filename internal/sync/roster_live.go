package sync

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"nextera-pulse/internal/store"
	"nextera-pulse/internal/zeroone"
)

// syncRoster makes the Pulse roster follow 01Edu:
//
//   - learners: members of the configured cohort labels (label + label_user)
//     who are enrolled in the event (event_user gives enrollment date and level);
//   - profiles: login, names and email from user;
//   - staff: users whose 01Edu role is "admin" or "admin campus".
//
// Nothing is deleted. A learner who leaves a label keeps their history.
// Staff roles are revoked only when the staff query succeeds and returns rows.
func syncRoster(ctx context.Context, db *store.DB, source LiveSource, cfg LiveCycleConfig, enrollments []zeroone.EventUser) StageReport {
	var report StageReport
	names := make([]string, 0, len(cfg.Cohorts))
	priority := map[string]int{}
	for i, cohort := range cfg.Cohorts {
		name := strings.TrimSpace(cohort.Label)
		if name == "" {
			continue
		}
		if _, seen := priority[name]; !seen {
			priority[name] = i
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		report.Error = "no cohort label names configured (PILOT_COHORTS)"
		return report
	}

	labels, err := source.CohortLabels(ctx, names)
	if err != nil {
		report.Error = stageError(source, "label", err)
		return report
	}
	labelCohort := map[int64]int{} // 01Edu label id -> index in cfg.Cohorts
	found := map[string]bool{}
	labelIDs := make([]int64, 0, len(labels))
	for _, label := range labels {
		idx, ok := priority[strings.TrimSpace(label.Name)]
		if !ok || label.ID <= 0 {
			continue
		}
		labelCohort[label.ID] = idx
		labelIDs = append(labelIDs, label.ID)
		found[strings.TrimSpace(label.Name)] = true
	}
	var missing []string
	for _, name := range names {
		if !found[name] {
			missing = append(missing, name)
		}
	}
	if len(labelIDs) == 0 {
		report.Error = "no 01Edu label matches PILOT_COHORTS: " + strings.Join(missing, ", ")
		return report
	}

	members, err := source.LabelUsers(ctx, labelIDs)
	if err != nil {
		report.Error = stageError(source, "label_user", err)
		return report
	}
	report.Rows = len(members)
	cohortOf := map[int64]int{} // 01Edu user id -> index in cfg.Cohorts
	for _, member := range members {
		idx, ok := labelCohort[member.LabelID]
		if !ok || member.UserID <= 0 {
			continue
		}
		if current, seen := cohortOf[member.UserID]; !seen || idx < current {
			cohortOf[member.UserID] = idx
		}
	}
	enrolled := make(map[int64]zeroone.EventUser, len(enrollments))
	for _, e := range enrollments {
		enrolled[e.UserID] = e
	}
	learnerIDs := make([]int64, 0, len(cohortOf))
	for userID := range cohortOf {
		if _, ok := enrolled[userID]; ok {
			learnerIDs = append(learnerIDs, userID)
		}
	}
	sort.Slice(learnerIDs, func(i, j int) bool { return learnerIDs[i] < learnerIDs[j] })

	// Staff: optional. A failure keeps the roles already stored.
	staffRole := map[int64]string{}
	staffErr := ""
	if staff, err := source.StaffRoles(ctx, StaffSourceRoles); err != nil {
		staffErr = stageError(source, "user.role", err)
	} else {
		for _, member := range staff {
			if member.ID > 0 {
				staffRole[member.ID] = strings.TrimSpace(member.Role)
			}
		}
		if len(staffRole) == 0 {
			staffErr = "staff query returned no admin accounts; existing staff roles kept"
		}
	}

	profileIDs := append([]int64{}, learnerIDs...)
	for id := range staffRole {
		if _, isLearner := cohortOf[id]; !isLearner || enrolled[id].UserID == 0 {
			profileIDs = append(profileIDs, id)
		}
	}
	profiles := map[int64]zeroone.LiveUser{}
	if len(profileIDs) > 0 {
		rows, err := source.UsersByID(ctx, profileIDs)
		if err != nil {
			report.Error = stageError(source, "user", err)
			return report
		}
		for _, row := range rows {
			if row.ID > 0 && strings.TrimSpace(row.Login) != "" {
				profiles[row.ID] = row
			}
		}
	}

	applied, err := applyRoster(ctx, db, cfg, labels, labelCohort, members, learnerIDs, cohortOf, enrolled, profiles, staffRole, staffErr == "")
	if err != nil {
		report.Error = "apply roster: " + err.Error()
		return report
	}
	report.OK = true
	report.Applied = applied
	report.Students = applied
	var notes []string
	if len(missing) > 0 {
		notes = append(notes, "labels not found: "+strings.Join(missing, ", "))
	}
	if staffErr != "" {
		notes = append(notes, staffErr)
	}
	report.Error = strings.Join(notes, "; ")
	return report
}

func applyRoster(ctx context.Context, db *store.DB, cfg LiveCycleConfig,
	labels []zeroone.LiveLabel, labelCohort map[int64]int, members []zeroone.LiveLabelUser,
	learnerIDs []int64, cohortOf map[int64]int, enrolled map[int64]zeroone.EventUser,
	profiles map[int64]zeroone.LiveUser, staffRole map[int64]string, revokeStaff bool) (int, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := time.Now().UTC()

	upsertPerson := db.Rebind(`INSERT INTO students (source_user_id, login, first_name, last_name, email, source_role, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (source_user_id) DO UPDATE SET
			login = EXCLUDED.login,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			email = EXCLUDED.email,
			source_role = CASE WHEN EXCLUDED.source_role = '' THEN students.source_role ELSE EXCLUDED.source_role END,
			updated_at = EXCLUDED.updated_at
		RETURNING id`)
	upsertEnrollment := db.Rebind(`INSERT INTO student_events (student_id, event_id, cohort_id, enrolled_at, current_level, last_synced_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (student_id, event_id) DO UPDATE SET
			cohort_id = EXCLUDED.cohort_id,
			enrolled_at = EXCLUDED.enrolled_at,
			current_level = EXCLUDED.current_level,
			last_synced_at = EXCLUDED.last_synced_at`)

	applied := 0
	for _, userID := range learnerIDs {
		profile, ok := profiles[userID]
		if !ok {
			continue // no readable profile: never invent a login
		}
		enrollment := enrolled[userID]
		role := staffRole[userID]
		if role == "" && revokeStaff {
			role = "user"
		}
		var studentID int64
		if err := tx.QueryRowContext(ctx, upsertPerson, userID, strings.TrimSpace(profile.Login),
			profile.FirstName, profile.LastName, profile.Email, role, enrollment.CreatedAt.UTC(), now).Scan(&studentID); err != nil {
			return 0, fmt.Errorf("upsert learner %d: %w", userID, err)
		}
		cohortID := cfg.Cohorts[cohortOf[userID]].ID
		if _, err := tx.ExecContext(ctx, upsertEnrollment, studentID, cfg.EventID, cohortID,
			enrollment.CreatedAt.UTC(), enrollment.Level, now); err != nil {
			return 0, fmt.Errorf("upsert enrollment %d: %w", userID, err)
		}
		applied++
	}

	for userID, role := range staffRole {
		profile, ok := profiles[userID]
		if !ok {
			if _, err := tx.ExecContext(ctx, db.Rebind(`UPDATE students SET source_role = ?, updated_at = ? WHERE source_user_id = ?`), role, now, userID); err != nil {
				return 0, err
			}
			continue
		}
		var ignored int64
		if err := tx.QueryRowContext(ctx, upsertPerson, userID, strings.TrimSpace(profile.Login),
			profile.FirstName, profile.LastName, profile.Email, role, now, now).Scan(&ignored); err != nil {
			return 0, fmt.Errorf("upsert staff %d: %w", userID, err)
		}
	}
	if revokeStaff {
		ids := make([]int64, 0, len(staffRole))
		for id := range staffRole {
			ids = append(ids, id)
		}
		marks := make([]string, len(ids))
		args := []any{now}
		for i, id := range ids {
			marks[i] = "?"
			args = append(args, id)
		}
		// People who are no longer 01Edu admins lose Pulse staff access.
		if _, err := tx.ExecContext(ctx, db.Rebind(`UPDATE students SET source_role = 'user', updated_at = ?
			WHERE source_role IN ('admin', 'admin campus') AND source_user_id NOT IN (`+strings.Join(marks, ",")+`)`), args...); err != nil {
			return 0, err
		}
	}

	labelQuery := db.Rebind(`INSERT INTO cohort_labels (source_label_id, event_id, name, description) VALUES (?, ?, ?, ?)
		ON CONFLICT (source_label_id) DO UPDATE SET event_id = EXCLUDED.event_id, name = EXCLUDED.name, description = EXCLUDED.description`)
	labelName := map[int64]string{}
	for _, label := range labels {
		if _, ok := labelCohort[label.ID]; !ok {
			continue
		}
		labelName[label.ID] = label.Name
		var eventID any
		if label.EventID != nil {
			eventID = *label.EventID
		}
		if _, err := tx.ExecContext(ctx, labelQuery, label.ID, eventID, label.Name, label.Description); err != nil {
			return 0, err
		}
	}
	memberQuery := db.Rebind(`INSERT INTO cohort_label_users (source_id, label_id, source_user_id, label_name) VALUES (?, ?, ?, ?)
		ON CONFLICT (source_id) DO UPDATE SET label_id = EXCLUDED.label_id, source_user_id = EXCLUDED.source_user_id, label_name = EXCLUDED.label_name`)
	for _, member := range members {
		name, ok := labelName[member.LabelID]
		if !ok || member.ID <= 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, memberQuery, member.ID, member.LabelID, member.UserID, name); err != nil {
			return 0, err
		}
	}
	return applied, tx.Commit()
}
