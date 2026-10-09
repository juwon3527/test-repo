package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
)

var schema = []string{
	`CREATE TABLE IF NOT EXISTS users (
		id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
		email VARCHAR(254) NOT NULL,
		name VARCHAR(80) NOT NULL,
		password_hash VARBINARY(72) NOT NULL,
		avatar_key VARCHAR(255) NOT NULL DEFAULT '',
		created_at DATETIME(3) NOT NULL,
		UNIQUE KEY users_email (email)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE IF NOT EXISTS sessions (
		token_hash BINARY(32) PRIMARY KEY,
		user_id BIGINT UNSIGNED NOT NULL,
		expires_at DATETIME(3) NOT NULL,
		KEY sessions_expires (expires_at),
		CONSTRAINT sessions_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
	) ENGINE=InnoDB`,
	`CREATE TABLE IF NOT EXISTS photos (
		id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
		user_id BIGINT UNSIGNED NOT NULL,
		image_key VARCHAR(255) NOT NULL,
		caption VARCHAR(140) NOT NULL DEFAULT '',
		created_at DATETIME(3) NOT NULL,
		CONSTRAINT photos_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
}

type mysqlStore struct {
	db *sql.DB
}

// openMySQL connects using a go-sql-driver DSN such as
// "user:pass@tcp(db:3306)/thebridge", waits up to 30s for the server to
// accept connections (it may still be starting in Docker Compose), and
// creates any missing tables.
func openMySQL(ctx context.Context, dsn string) (*mysqlStore, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse MYSQL_DSN: %w", err)
	}
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(3 * time.Minute)

	deadline := time.Now().Add(30 * time.Second)
	for {
		err = db.PingContext(ctx)
		if err == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to MySQL: %w", err)
	}
	for _, statement := range schema {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("create schema: %w", err)
		}
	}
	return &mysqlStore{db: db}, nil
}

func (s *mysqlStore) CreateUser(ctx context.Context, email, name string, passwordHash []byte) (User, error) {
	user := User{Email: email, Name: name, PasswordHash: passwordHash, CreatedAt: time.Now().UTC().Truncate(time.Millisecond)}
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO users (email, name, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		user.Email, user.Name, user.PasswordHash, user.CreatedAt)
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 { // ER_DUP_ENTRY
		return User{}, ErrEmailTaken
	}
	if err != nil {
		return User{}, err
	}
	user.ID, err = result.LastInsertId()
	return user, err
}

const userColumns = `u.id, u.email, u.name, u.password_hash, u.avatar_key, u.created_at`

func scanUser(row *sql.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.AvatarKey, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *mysqlStore) UserByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users u WHERE u.email = ?`, email))
}

func (s *mysqlStore) SetAvatar(ctx context.Context, userID int64, key string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET avatar_key = ? WHERE id = ?`, key, userID)
	return err
}

func (s *mysqlStore) CreateSession(ctx context.Context, tokenHash []byte, userID int64, expires time.Time) error {
	// Opportunistically clear out expired sessions so the table doesn't grow forever.
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, time.Now().UTC()); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		tokenHash, userID, expires.UTC())
	return err
}

func (s *mysqlStore) SessionUser(ctx context.Context, tokenHash []byte, now time.Time) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, now.UTC()))
}

func (s *mysqlStore) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *mysqlStore) AddPhoto(ctx context.Context, userID int64, key, caption string) (Photo, error) {
	photo := Photo{UserID: userID, ImageKey: key, Caption: caption, CreatedAt: time.Now().UTC().Truncate(time.Millisecond)}
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO photos (user_id, image_key, caption, created_at) VALUES (?, ?, ?, ?)`,
		userID, key, caption, photo.CreatedAt)
	if err != nil {
		return Photo{}, err
	}
	if photo.ID, err = result.LastInsertId(); err != nil {
		return Photo{}, err
	}
	err = s.db.QueryRowContext(ctx, `SELECT name FROM users WHERE id = ?`, userID).Scan(&photo.Author)
	return photo, err
}

func (s *mysqlStore) ListPhotos(ctx context.Context, limit int) ([]Photo, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT p.id, p.user_id, u.name, p.image_key, p.caption, p.created_at
		 FROM photos p JOIN users u ON u.id = p.user_id
		 ORDER BY p.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	photos := []Photo{}
	for rows.Next() {
		var p Photo
		if err := rows.Scan(&p.ID, &p.UserID, &p.Author, &p.ImageKey, &p.Caption, &p.CreatedAt); err != nil {
			return nil, err
		}
		photos = append(photos, p)
	}
	return photos, rows.Err()
}
