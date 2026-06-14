package db

import (
	"context"
	"database/sql"
	"errors"
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
			logger.Info("Successfully connected to database in Chat Service")
			return &DB{db}, nil
		}
		logger.Error("Database ping failed (attempt %d/5): %v", i, err)
		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("failed to connect to database after retries: %w", err)
}

type Room struct {
	ID        string    `json:"id"`
	RoomType  string    `json:"room_type"` // "direct" or "group"
	RoomName  string    `json:"room_name"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

type RoomMember struct {
	RoomID      string    `json:"room_id"`
	UserID      string    `json:"user_id"`
	JoinedAt    time.Time `json:"joined_at"`
	DisplayName string    `json:"display_name"`
	Username    string    `json:"username"`
}

type Message struct {
	ID          string    `json:"id"`
	RoomID      string    `json:"room_id"`
	SenderID    string    `json:"sender_id"`
	Content     string    `json:"content"`
	MessageType string    `json:"message_type"` // "text", "image", "system"
	CreatedAt   time.Time `json:"created_at"`
}

// FindDirectRoom checks if a direct room already exists between two users (Duplicate DM Prevention)
func (db *DB) FindDirectRoom(ctx context.Context, userA, userB string) (string, error) {
	query := `
		SELECT r.id
		FROM rooms r
		JOIN room_members rm1 ON r.id = rm1.room_id
		JOIN room_members rm2 ON r.id = rm2.room_id
		WHERE r.room_type = 'direct'
		  AND rm1.user_id = $1
		  AND rm2.user_id = $2
		  AND (SELECT COUNT(*) FROM room_members WHERE room_id = r.id) = 2
		LIMIT 1
	`
	var roomID string
	err := db.QueryRowContext(ctx, query, userA, userB).Scan(&roomID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return roomID, nil
}

// CreateRoom creates a room and registers its members inside a SQL transaction
func (db *DB) CreateRoom(ctx context.Context, roomType, roomName, createdBy string, memberIDs []string) (*Room, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	queryRoom := `
		INSERT INTO rooms (room_type, room_name, created_by)
		VALUES ($1, $2, $3)
		RETURNING id, room_type, room_name, created_by, created_at
	`
	var r Room
	err = tx.QueryRowContext(ctx, queryRoom, roomType, roomName, createdBy).Scan(&r.ID, &r.RoomType, &r.RoomName, &r.CreatedBy, &r.CreatedAt)
	if err != nil {
		return nil, err
	}

	queryMember := `
		INSERT INTO room_members (room_id, user_id)
		VALUES ($1, $2)
	`
	for _, mID := range memberIDs {
		_, err = tx.ExecContext(ctx, queryMember, r.ID, mID)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &r, nil
}

// IsMember checks if a user is authorized in the room
func (db *DB) IsMember(ctx context.Context, roomID, userID string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM room_members
			WHERE room_id = $1 AND user_id = $2
		)
	`
	var exists bool
	err := db.QueryRowContext(ctx, query, roomID, userID).Scan(&exists)
	return exists, err
}

// SaveMessage stores a message in database after performing membership validation
func (db *DB) SaveMessage(ctx context.Context, roomID, senderID, content, messageType string) (*Message, error) {
	isMem, err := db.IsMember(ctx, roomID, senderID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMem {
		return nil, errors.New("unauthorized: sender is not a member of the room")
	}

	query := `
		INSERT INTO messages (room_id, sender_id, content, message_type)
		VALUES ($1, $2, $3, $4)
		RETURNING id, room_id, sender_id, content, message_type, created_at
	`
	var m Message
	err = db.QueryRowContext(ctx, query, roomID, senderID, content, messageType).Scan(&m.ID, &m.RoomID, &m.SenderID, &m.Content, &m.MessageType, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetRoomsForUser queries all rooms a user is registered in
func (db *DB) GetRoomsForUser(ctx context.Context, userID string) ([]*Room, error) {
	query := `
		SELECT r.id, r.room_type, COALESCE(r.room_name, ''), r.created_by, r.created_at
		FROM rooms r
		JOIN room_members rm ON r.id = rm.room_id
		WHERE rm.user_id = $1
		ORDER BY r.created_at DESC
	`
	rows, err := db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []*Room
	for rows.Next() {
		var r Room
		if err := rows.Scan(&r.ID, &r.RoomType, &r.RoomName, &r.CreatedBy, &r.CreatedAt); err != nil {
			return nil, err
		}
		rooms = append(rooms, &r)
	}
	return rooms, nil
}

// GetRoomMembers queries all users inside a chat room
func (db *DB) GetRoomMembers(ctx context.Context, roomID string) ([]*RoomMember, error) {
	query := `
		SELECT rm.room_id, rm.user_id, rm.joined_at, u.display_name, u.username
		FROM room_members rm
		JOIN users u ON rm.user_id = u.id
		WHERE rm.room_id = $1
		ORDER BY rm.joined_at ASC
	`
	rows, err := db.QueryContext(ctx, query, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*RoomMember
	for rows.Next() {
		var m RoomMember
		if err := rows.Scan(&m.RoomID, &m.UserID, &m.JoinedAt, &m.DisplayName, &m.Username); err != nil {
			return nil, err
		}
		members = append(members, &m)
	}
	return members, nil
}

// GetMessagesForRoom queries historical messages descending (newest first)
func (db *DB) GetMessagesForRoom(ctx context.Context, roomID string, limit int, beforeTime time.Time) ([]*Message, error) {
	var rows *sql.Rows
	var err error
	
	if beforeTime.IsZero() {
		query := `
			SELECT id, room_id, sender_id, content, message_type, created_at
			FROM messages
			WHERE room_id = $1
			ORDER BY created_at DESC
			LIMIT $2
		`
		rows, err = db.QueryContext(ctx, query, roomID, limit)
	} else {
		query := `
			SELECT id, room_id, sender_id, content, message_type, created_at
			FROM messages
			WHERE room_id = $1 AND created_at < $2
			ORDER BY created_at DESC
			LIMIT $3
		`
		rows, err = db.QueryContext(ctx, query, roomID, beforeTime, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.RoomID, &m.SenderID, &m.Content, &m.MessageType, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, &m)
	}
	return messages, nil
}

// GetRoomByID queries room information by ID
func (db *DB) GetRoomByID(ctx context.Context, id string) (*Room, error) {
	query := `
		SELECT id, room_type, COALESCE(room_name, ''), created_by, created_at
		FROM rooms
		WHERE id = $1
	`
	var r Room
	err := db.QueryRowContext(ctx, query, id).Scan(&r.ID, &r.RoomType, &r.RoomName, &r.CreatedBy, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &r, err
}
