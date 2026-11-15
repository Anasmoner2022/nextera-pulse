package store

import (
	"context"
	"database/sql/driver"
	"errors"
	"log"
	"time"
)

// Advisory lock keys. Each background job has its own key so a live sync and
// a snapshot refresh do not block each other, but two instances of the same
// job never overlap (also across API replicas and cmd/worker).
const (
	LockLiveSync  int64 = 0x50554C53_00000001 // "PULS" · live 01Edu sync cycle
	LockSnapshots int64 = 0x50554C53_00000002 // "PULS" · daily pace snapshot job
)

// ErrLockHeld means another session holds the advisory lock.
var ErrLockHeld = errors.New("another instance holds the lock")

// WithAdvisoryLock runs fn while holding the session-level advisory lock key on
// a dedicated connection. With wait=false it uses pg_try_advisory_lock and
// returns ErrLockHeld at once when another session holds the key; with
// wait=true it blocks (pg_advisory_lock) until the lock is free or ctx ends.
//
// The lock is released when fn returns, returns an error or panics, and when
// ctx is cancelled: the unlock runs on a fresh context. If the unlock itself
// fails, the connection is discarded, which ends the session and so releases
// the lock in PostgreSQL; it is never returned to the pool still locked.
func (db *DB) WithAdvisoryLock(ctx context.Context, key int64, wait bool, fn func(context.Context) error) error {
	if db.DriverName == "sqlite" {
		// SQLite has no session advisory locks. One process is the fallback.
		return fn(ctx)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	locked := false
	defer func() {
		if locked {
			unlockCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			var released bool
			uerr := conn.QueryRowContext(unlockCtx, `SELECT pg_advisory_unlock($1)`, key).Scan(&released)
			cancel()
			if uerr != nil || !released {
				log.Printf("advisory lock %d: unlock failed (%v); closing the session to release it", key, uerr)
				_ = conn.Raw(func(any) error { return driver.ErrBadConn })
			}
		}
		conn.Close()
	}()

	if wait {
		if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, key); err != nil {
			// A cancelled wait may still have been granted; drop the session to be sure.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
			return err
		}
	} else {
		var got bool
		if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&got); err != nil {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
			return err
		}
		if !got {
			return ErrLockHeld
		}
	}
	locked = true
	return fn(ctx)
}

// AdvisoryLockHeld reports whether any session currently holds key (for tests and diagnostics).
func (db *DB) AdvisoryLockHeld(ctx context.Context, key int64) (bool, error) {
	if db.DriverName == "sqlite" {
		return false, nil
	}
	var held bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM pg_locks WHERE locktype = 'advisory' AND granted
		  AND database = (SELECT oid FROM pg_database WHERE datname = current_database())
		  AND ((classid::bigint << 32) | objid::bigint) = $1)`, key).Scan(&held)
	return held, err
}
