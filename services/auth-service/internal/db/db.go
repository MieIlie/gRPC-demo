package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"shared/logger"
	_ "github.com/lib/pq"
)

type DB struct {
	*sql.DB
}

func Connect(connStr string) (*DB, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	for i := 1; i <= 5; i++ {
		err = db.Ping()
		if err == nil {
			logger.Info("Successfully connected to database")
			return &DB{db}, nil
		}
		logger.Error("Database ping failed (attempt %d/5): %v", i, err)
		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("failed to connect to database after retries: %w", err)
}

type User struct {
	ID           string
	Username     string
	PasswordHash string
	DisplayName  string
	AvatarURL    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (db *DB) CreateUser(ctx context.Context, username, passwordHash, displayName, avatarURL string) (*User, error) {
	query := `
		INSERT INTO users (username, password_hash, display_name, avatar_url)
		VALUES ($1, $2, $3, $4)
		RETURNING id, username, password_hash, display_name, COALESCE(avatar_url, ''), created_at, updated_at
	`
	row := db.QueryRowContext(ctx, query, username, passwordHash, displayName, avatarURL)

	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	query := `
		SELECT id, username, password_hash, display_name, COALESCE(avatar_url, ''), created_at, updated_at
		FROM users
		WHERE username = $1
	`
	row := db.QueryRowContext(ctx, query, username)

	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) CreateSession(ctx context.Context, userID, refreshToken string, expiresAt time.Time) error {
	query := `
		INSERT INTO user_sessions (user_id, refresh_token, expires_at)
		VALUES ($1, $2, $3)
	`
	_, err := db.ExecContext(ctx, query, userID, refreshToken, expiresAt)
	return err
}

func (db *DB) DeleteSession(ctx context.Context, refreshToken string) error {
	query := `
		DELETE FROM user_sessions
		WHERE refresh_token = $1
	`
	_, err := db.ExecContext(ctx, query, refreshToken)
	return err
}
