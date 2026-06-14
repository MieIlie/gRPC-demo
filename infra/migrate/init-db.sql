CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    avatar_url TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_type VARCHAR(20) NOT NULL,
    room_name VARCHAR(100),
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT room_type_check
    CHECK (
        room_type IN ('direct', 'group')
    )
);

CREATE TABLE room_members (
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at TIMESTAMP NOT NULL DEFAULT NOW(),

    PRIMARY KEY (room_id, user_id)
);

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    sender_id UUID NOT NULL REFERENCES users(id),
    content TEXT NOT NULL,
    message_type VARCHAR(20) NOT NULL DEFAULT 'text',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT message_type_check
    CHECK (
        message_type IN (
            'text',
            'image',
            'system'
        )
    )
);

CREATE TABLE user_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE call_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id UUID REFERENCES rooms(id) ON DELETE SET NULL,
    caller_id UUID NOT NULL REFERENCES users(id),
    receiver_id UUID NOT NULL REFERENCES users(id),
    call_type VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL,
    started_at TIMESTAMP,
    ended_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT call_type_check
    CHECK (
        call_type IN (
            'voice',
            'video'
        )
    ),

    CONSTRAINT call_status_check
    CHECK (
        status IN (
            'pending',
            'accepted',
            'rejected',
            'ended',
            'missed'
        )
    )
);

CREATE INDEX idx_messages_room_id
ON messages(room_id);

CREATE INDEX idx_messages_created_at
ON messages(created_at);

CREATE INDEX idx_room_members_user_id
ON room_members(user_id);

CREATE INDEX idx_call_sessions_room_id
ON call_sessions(room_id);

CREATE INDEX idx_call_sessions_created_at
ON call_sessions(created_at);

-- Seed test users (password is 'password123')
INSERT INTO users (id, username, password_hash, display_name) VALUES
('11111111-1111-1111-1111-111111111111', 'alice', '$2a$10$6fxT3J2ic0JFeCES6HPaze67je.5CgRLOZ2rWye4k9Cb43Mct4koK', 'Alice Henderson'),
('22222222-2222-2222-2222-222222222222', 'bob', '$2a$10$6fxT3J2ic0JFeCES6HPaze67je.5CgRLOZ2rWye4k9Cb43Mct4koK', 'Bob Vance')
ON CONFLICT (username) DO NOTHING;

-- Seed General Room
INSERT INTO rooms (id, room_type, room_name, created_by) VALUES
('a3333333-3333-3333-3333-333333333333', 'group', 'General Room', '11111111-1111-1111-1111-111111111111')
ON CONFLICT (id) DO NOTHING;

-- Seed Room Members for General Room
INSERT INTO room_members (room_id, user_id) VALUES
('a3333333-3333-3333-3333-333333333333', '11111111-1111-1111-1111-111111111111'),
('a3333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222222')
ON CONFLICT (room_id, user_id) DO NOTHING;

-- Trigger to auto-add new users to General Room
CREATE OR REPLACE FUNCTION add_new_user_to_general_room()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO room_members (room_id, user_id)
    VALUES ('a3333333-3333-3333-3333-333333333333', NEW.id)
    ON CONFLICT (room_id, user_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER trg_add_new_user_to_general_room
AFTER INSERT ON users
FOR EACH ROW
EXECUTE FUNCTION add_new_user_to_general_room();

