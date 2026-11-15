package store

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"nextera-pulse/internal/store/pgtest"
)

// testLockKey avoids the production keys: advisory locks are database-wide
// and other test packages run at the same time.
func testLockKey() int64 { return 0x7E570000_00000000 | rand.Int63n(1<<40) }

func lockDB(t *testing.T) *DB {
	t.Helper()
	db, err := Connect(pgtest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func assertReleased(t *testing.T, db *DB, key int64) {
	t.Helper()
	held, err := db.AdvisoryLockHeld(context.Background(), key)
	if err != nil || held {
		t.Fatalf("lock %d still held (%v)", key, err)
	}
	ran := false
	if err := db.WithAdvisoryLock(context.Background(), key, false, func(context.Context) error { ran = true; return nil }); err != nil || !ran {
		t.Fatalf("re-acquire after release: ran=%v err=%v", ran, err)
	}
}

func TestAdvisoryLockSkipsWhileHeldAndReleasesAfterwards(t *testing.T) {
	db, key, ctx := lockDB(t), testLockKey(), context.Background()
	inside, release := make(chan struct{}), make(chan struct{})
	done := make(chan error)
	go func() {
		done <- db.WithAdvisoryLock(ctx, key, false, func(context.Context) error { close(inside); <-release; return nil })
	}()
	<-inside
	if held, _ := db.AdvisoryLockHeld(ctx, key); !held {
		t.Fatal("lock should be visible in pg_locks")
	}
	ran := false
	if err := db.WithAdvisoryLock(ctx, key, false, func(context.Context) error { ran = true; return nil }); !errors.Is(err, ErrLockHeld) || ran {
		t.Fatalf("second holder: ran=%v err=%v, want ErrLockHeld", ran, err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	assertReleased(t, db, key)
}

func TestAdvisoryLockIsReleasedOnErrorPanicAndCancel(t *testing.T) {
	db, ctx := lockDB(t), context.Background()

	key := testLockKey()
	boom := errors.New("boom")
	if err := db.WithAdvisoryLock(ctx, key, false, func(context.Context) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	assertReleased(t, db, key)

	key = testLockKey()
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic should propagate")
			}
		}()
		_ = db.WithAdvisoryLock(ctx, key, false, func(context.Context) error { panic("cycle crashed") })
	}()
	assertReleased(t, db, key)

	key = testLockKey()
	cancelCtx, cancel := context.WithCancel(ctx)
	err := db.WithAdvisoryLock(cancelCtx, key, false, func(c context.Context) error {
		cancel()
		<-c.Done()
		return c.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	assertReleased(t, db, key)
}

func TestAdvisoryLockWaitBlocksUntilFree(t *testing.T) {
	db, key, ctx := lockDB(t), testLockKey(), context.Background()
	inside := make(chan struct{})
	go func() {
		_ = db.WithAdvisoryLock(ctx, key, false, func(context.Context) error { close(inside); time.Sleep(200 * time.Millisecond); return nil })
	}()
	<-inside
	start := time.Now()
	if err := db.WithAdvisoryLock(ctx, key, true, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < 100*time.Millisecond {
		t.Fatal("wait=true should have waited for the holder")
	}
	// A cancelled wait gives up and leaves nothing locked.
	inside2, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		_ = db.WithAdvisoryLock(ctx, key, false, func(context.Context) error { close(inside2); <-release; return nil })
		close(finished)
	}()
	<-inside2
	waitCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	if err := db.WithAdvisoryLock(waitCtx, key, true, func(context.Context) error { return nil }); err == nil {
		t.Fatal("cancelled wait should fail")
	}
	close(release)
	<-finished
	assertReleased(t, db, key)
}
