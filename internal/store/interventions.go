package store

import (
	"context"
	"database/sql"
	"errors"
)

// ErrInterventionNotFound means no visible (not deleted) follow-up has that id.
var ErrInterventionNotFound = errors.New("follow-up not found")

// InterventionAuthor returns the author (interventions.mentor_id) of a
// follow-up that has not been deleted.
func (db *DB) InterventionAuthor(ctx context.Context, id int64) (int64, error) {
	var author int64
	err := db.QueryRowContext(ctx, `SELECT mentor_id FROM interventions WHERE id = $1 AND deleted_at IS NULL`, id).Scan(&author)
	if err == sql.ErrNoRows {
		return 0, ErrInterventionNotFound
	}
	return author, err
}

// SoftDeleteIntervention hides a follow-up from every list, recording who
// deleted it and when. The row stays for the record.
func (db *DB) SoftDeleteIntervention(ctx context.Context, id, deletedBy int64) error {
	res, err := db.ExecContext(ctx, `UPDATE interventions SET deleted_at = NOW(), deleted_by = $2
		WHERE id = $1 AND deleted_at IS NULL`, id, nullableID(deletedBy))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrInterventionNotFound
	}
	return nil
}
