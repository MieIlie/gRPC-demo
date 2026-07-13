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
			logger.Info("Successfully connected to database in Call Service")
			return &DB{db}, nil
		}
		logger.Error("Database ping failed (attempt %d/5): %v", i, err)
		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("failed to connect to database after retries: %w", err)
}

type CallSession struct {
	ID         string         `json:"id"`
	RoomID     sql.NullString `json:"room_id"`
	CallerID   string         `json:"caller_id"`
	ReceiverID string         `json:"receiver_id"`
	CallType   string         `json:"call_type"`
	Status     string         `json:"status"`
	StartedAt  sql.NullTime   `json:"started_at"`
	EndedAt    sql.NullTime   `json:"ended_at"`
	CreatedAt  time.Time      `json:"created_at"`
}

func (db *DB) StartCall(ctx context.Context, roomID, callerID, receiverID, callType string) (*CallSession, error) {
	query := `
		INSERT INTO call_sessions (room_id, caller_id, receiver_id, call_type, status)
		VALUES (
			CASE WHEN $1 = '' THEN NULL ELSE $1::UUID END,
			$2, $3, $4, 'pending'
		)
		RETURNING id, room_id, caller_id, receiver_id, call_type, status, started_at, ended_at, created_at
	`
	var s CallSession
	err := db.QueryRowContext(ctx, query, roomID, callerID, receiverID, callType).Scan(
		&s.ID, &s.RoomID, &s.CallerID, &s.ReceiverID, &s.CallType, &s.Status, &s.StartedAt, &s.EndedAt, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (db *DB) AcceptCall(ctx context.Context, callID string) (*CallSession, error) {
	query := `
		UPDATE call_sessions
		SET status = 'accepted', started_at = NOW()
		WHERE id = $1
		RETURNING id, room_id, caller_id, receiver_id, call_type, status, started_at, ended_at, created_at
	`
	var s CallSession
	err := db.QueryRowContext(ctx, query, callID).Scan(
		&s.ID, &s.RoomID, &s.CallerID, &s.ReceiverID, &s.CallType, &s.Status, &s.StartedAt, &s.EndedAt, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (db *DB) RejectCall(ctx context.Context, callID string) (*CallSession, error) {
	query := `
		UPDATE call_sessions
		SET status = 'rejected'
		WHERE id = $1
		RETURNING id, room_id, caller_id, receiver_id, call_type, status, started_at, ended_at, created_at
	`
	var s CallSession
	err := db.QueryRowContext(ctx, query, callID).Scan(
		&s.ID, &s.RoomID, &s.CallerID, &s.ReceiverID, &s.CallType, &s.Status, &s.StartedAt, &s.EndedAt, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (db *DB) EndCall(ctx context.Context, callID string) (*CallSession, error) {
	query := `
		UPDATE call_sessions
		SET status = 'ended', ended_at = NOW()
		WHERE id = $1
		RETURNING id, room_id, caller_id, receiver_id, call_type, status, started_at, ended_at, created_at
	`
	var s CallSession
	err := db.QueryRowContext(ctx, query, callID).Scan(
		&s.ID, &s.RoomID, &s.CallerID, &s.ReceiverID, &s.CallType, &s.Status, &s.StartedAt, &s.EndedAt, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (db *DB) GetCallSession(ctx context.Context, callID string) (*CallSession, error) {
	query := `
		SELECT id, room_id, caller_id, receiver_id, call_type, status, started_at, ended_at, created_at
		FROM call_sessions
		WHERE id = $1
	`
	var s CallSession
	err := db.QueryRowContext(ctx, query, callID).Scan(
		&s.ID, &s.RoomID, &s.CallerID, &s.ReceiverID, &s.CallType, &s.Status, &s.StartedAt, &s.EndedAt, &s.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &s, err
}
