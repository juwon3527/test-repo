package main

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrEmailTaken = errors.New("email already registered")
)

type User struct {
	ID           int64
	Email        string
	Name         string
	PasswordHash []byte
	AvatarKey    string
	CreatedAt    time.Time
}

type Photo struct {
	ID        int64
	UserID    int64
	Author    string
	ImageKey  string
	Caption   string
	CreatedAt time.Time
}

// Store persists users, login sessions and gallery photos.
// Sessions are looked up by the SHA-256 of their token, so a leaked
// database can't be used to hijack live sessions.
type Store interface {
	CreateUser(ctx context.Context, email, name string, passwordHash []byte) (User, error)
	UserByEmail(ctx context.Context, email string) (User, error)
	SetAvatar(ctx context.Context, userID int64, key string) error

	CreateSession(ctx context.Context, tokenHash []byte, userID int64, expires time.Time) error
	SessionUser(ctx context.Context, tokenHash []byte, now time.Time) (User, error)
	DeleteSession(ctx context.Context, tokenHash []byte) error

	AddPhoto(ctx context.Context, userID int64, key, caption string) (Photo, error)
	ListPhotos(ctx context.Context, limit int) ([]Photo, error)
}

// memoryStore keeps everything in process memory. It backs the tests and
// local development when no MYSQL_DSN is configured; data is lost on restart.
type memoryStore struct {
	mu       sync.Mutex
	users    []User
	sessions map[string]memorySession
	photos   []Photo
}

type memorySession struct {
	userID  int64
	expires time.Time
}

func newMemoryStore() *memoryStore {
	return &memoryStore{sessions: map[string]memorySession{}}
}

func (m *memoryStore) CreateUser(_ context.Context, email, name string, passwordHash []byte) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Email == email {
			return User{}, ErrEmailTaken
		}
	}
	user := User{ID: int64(len(m.users) + 1), Email: email, Name: name, PasswordHash: passwordHash, CreatedAt: time.Now().UTC()}
	m.users = append(m.users, user)
	return user, nil
}

func (m *memoryStore) UserByEmail(_ context.Context, email string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return User{}, ErrNotFound
}

func (m *memoryStore) SetAvatar(_ context.Context, userID int64, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if userID < 1 || int(userID) > len(m.users) {
		return ErrNotFound
	}
	m.users[userID-1].AvatarKey = key
	return nil
}

func (m *memoryStore) CreateSession(_ context.Context, tokenHash []byte, userID int64, expires time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[string(tokenHash)] = memorySession{userID: userID, expires: expires}
	return nil
}

func (m *memoryStore) SessionUser(_ context.Context, tokenHash []byte, now time.Time) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[string(tokenHash)]
	if !ok || !now.Before(s.expires) {
		return User{}, ErrNotFound
	}
	return m.users[s.userID-1], nil
}

func (m *memoryStore) DeleteSession(_ context.Context, tokenHash []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, string(tokenHash))
	return nil
}

func (m *memoryStore) AddPhoto(_ context.Context, userID int64, key, caption string) (Photo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	photo := Photo{ID: int64(len(m.photos) + 1), UserID: userID, Author: m.users[userID-1].Name, ImageKey: key, Caption: caption, CreatedAt: time.Now().UTC()}
	m.photos = append(m.photos, photo)
	return photo, nil
}

func (m *memoryStore) ListPhotos(_ context.Context, limit int) ([]Photo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	photos := slices.Clone(m.photos)
	slices.Reverse(photos)
	if len(photos) > limit {
		photos = photos[:limit]
	}
	return photos, nil
}
