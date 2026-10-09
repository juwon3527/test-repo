package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestMemoryStore(t *testing.T) {
	testStore(t, newMemoryStore())
}

// TestMySQLStore runs against a real server when TEST_MYSQL_DSN is set, e.g.
// TEST_MYSQL_DSN="root:secret@tcp(127.0.0.1:3306)/thebridge_test" go test ./...
func TestMySQLStore(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN not set")
	}
	store, err := openMySQL(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.db.Close() })
	testStore(t, store)
}

func testStore(t *testing.T, store Store) {
	ctx := context.Background()
	// Unique per run so the MySQL test can reuse a database.
	email := fmt.Sprintf("fan-%d@example.com", time.Now().UnixNano())

	user, err := store.CreateUser(ctx, email, "Blue Fan", []byte("hash"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateUser(ctx, email, "Someone Else", []byte("hash")); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("duplicate email error = %v, want ErrEmailTaken", err)
	}
	found, err := store.UserByEmail(ctx, email)
	if err != nil || found.ID != user.ID || !bytes.Equal(found.PasswordHash, []byte("hash")) {
		t.Fatalf("UserByEmail = %+v, %v", found, err)
	}
	if _, err := store.UserByEmail(ctx, "missing-"+email); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing user error = %v", err)
	}

	if err := store.SetAvatar(ctx, user.ID, "avatars/a.png"); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	live, expired := bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32)
	live[0], expired[0] = byte(now.UnixNano()), byte(now.UnixNano()+1)
	if err := store.CreateSession(ctx, live, user.ID, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(ctx, expired, user.ID, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	sessionUser, err := store.SessionUser(ctx, live, now)
	if err != nil || sessionUser.ID != user.ID || sessionUser.AvatarKey != "avatars/a.png" {
		t.Fatalf("SessionUser = %+v, %v", sessionUser, err)
	}
	if _, err := store.SessionUser(ctx, expired, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired session error = %v", err)
	}
	if err := store.DeleteSession(ctx, live); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SessionUser(ctx, live, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted session error = %v", err)
	}

	first, err := store.AddPhoto(ctx, user.ID, "gallery/1.png", "first")
	if err != nil || first.Author != "Blue Fan" {
		t.Fatalf("AddPhoto = %+v, %v", first, err)
	}
	second, _ := store.AddPhoto(ctx, user.ID, "gallery/2.png", "second")
	photos, err := store.ListPhotos(ctx, 2)
	if err != nil || len(photos) != 2 || photos[0].ID != second.ID || photos[1].Caption != "first" {
		t.Fatalf("ListPhotos = %+v, %v", photos, err)
	}
}
