package server

import (
	"context"
	"fmt"
	"gen/chat"
	"chat-service/internal/db"
	"io"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ChatServer struct {
	chat.UnimplementedChatServiceServer
	db *db.DB
}

func NewChatServer(database *db.DB) *ChatServer {
	return &ChatServer{db: database}
}

// Convert RoomType proto enum to DB string
func roomTypeToDB(rt chat.RoomType) string {
	if rt == chat.RoomType_DIRECT {
		return "direct"
	}
	return "group"
}

// Convert DB string to RoomType proto enum
func roomTypeToProto(rt string) chat.RoomType {
	if rt == "direct" {
		return chat.RoomType_DIRECT
	}
	return chat.RoomType_GROUP
}

// Convert MessageType proto enum to DB string
func messageTypeToDB(mt chat.MessageType) string {
	switch mt {
	case chat.MessageType_IMAGE:
		return "image"
	case chat.MessageType_SYSTEM:
		return "system"
	default:
		return "text"
	}
}

// Convert DB string to MessageType proto enum
func messageTypeToProto(mt string) chat.MessageType {
	switch mt {
	case "image":
		return chat.MessageType_IMAGE
	case "system":
		return chat.MessageType_SYSTEM
	default:
		return chat.MessageType_TEXT
	}
}

func (s *ChatServer) CreateRoom(ctx context.Context, req *chat.CreateRoomRequest) (*chat.CreateRoomResponse, error) {
	createdBy := req.GetCreatedBy()
	if createdBy == "" {
		return nil, status.Error(codes.InvalidArgument, "creator user_id is required")
	}

	roomType := req.GetRoomType()
	members := req.GetMemberIds()

	// Ensure creator is in the members list
	hasCreator := false
	for _, m := range members {
		if m == createdBy {
			hasCreator = true
			break
		}
	}
	if !hasCreator {
		members = append(members, createdBy)
	}

	// 1. Direct Room Reuse (Duplicate DM Prevention)
	if roomType == chat.RoomType_DIRECT {
		if len(members) != 2 {
			return nil, status.Error(codes.InvalidArgument, "direct message rooms must contain exactly 2 members")
		}

		existingRoomID, err := s.db.FindDirectRoom(ctx, members[0], members[1])
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to check existing direct rooms: %v", err)
		}

		if existingRoomID != "" {
			roomData, err := s.db.GetRoomByID(ctx, existingRoomID)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to fetch existing direct room: %v", err)
			}
			if roomData != nil {
				return &chat.CreateRoomResponse{
					Room: &chat.Room{
						Id:        roomData.ID,
						RoomType:  roomTypeToProto(roomData.RoomType),
						RoomName:  roomData.RoomName,
						CreatedBy: roomData.CreatedBy,
						CreatedAt: timestamppb.New(roomData.CreatedAt),
					},
				}, nil
			}
		}
	}

	// 2. Create new room in transactional sequence
	r, err := s.db.CreateRoom(ctx, roomTypeToDB(roomType), req.GetRoomName(), createdBy, members)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create room: %v", err)
	}

	return &chat.CreateRoomResponse{
		Room: &chat.Room{
			Id:        r.ID,
			RoomType:  roomTypeToProto(r.RoomType),
			RoomName:  r.RoomName,
			CreatedBy: r.CreatedBy,
			CreatedAt: timestamppb.New(r.CreatedAt),
		},
	}, nil
}

func (s *ChatServer) GetRooms(ctx context.Context, req *chat.GetRoomsRequest) (*chat.GetRoomsResponse, error) {
	userID := req.GetUserId()
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	dbRooms, err := s.db.GetRoomsForUser(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to query rooms: %v", err)
	}

	protoRooms := make([]*chat.Room, 0, len(dbRooms))
	for _, r := range dbRooms {
		protoRooms = append(protoRooms, &chat.Room{
			Id:        r.ID,
			RoomType:  roomTypeToProto(r.RoomType),
			RoomName:  r.RoomName,
			CreatedBy: r.CreatedBy,
			CreatedAt: timestamppb.New(r.CreatedAt),
		})
	}

	return &chat.GetRoomsResponse{Rooms: protoRooms}, nil
}

func (s *ChatServer) SendMessage(ctx context.Context, req *chat.SendMessageRequest) (*chat.SendMessageResponse, error) {
	roomID := req.GetRoomId()
	senderID := req.GetSenderId()
	content := req.GetContent()

	if roomID == "" || senderID == "" || content == "" {
		return nil, status.Error(codes.InvalidArgument, "room_id, sender_id, and content are required")
	}

	dbMsg, err := s.db.SaveMessage(ctx, roomID, senderID, content, messageTypeToDB(req.GetMessageType()))
	if err != nil {
		// Return PermissionDenied if it is a membership authorization error
		if err.Error() == "unauthorized: sender is not a member of the room" {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "failed to save message: %v", err)
	}

	return &chat.SendMessageResponse{
		Message: &chat.Message{
			Id:          dbMsg.ID,
			RoomId:      dbMsg.RoomID,
			SenderId:    dbMsg.SenderID,
			Content:     dbMsg.Content,
			MessageType: messageTypeToProto(dbMsg.MessageType),
			CreatedAt:   timestamppb.New(dbMsg.CreatedAt),
		},
	}, nil
}

func (s *ChatServer) GetMessages(ctx context.Context, req *chat.GetMessagesRequest) (*chat.GetMessagesResponse, error) {
	roomID := req.GetRoomId()
	if roomID == "" {
		return nil, status.Error(codes.InvalidArgument, "room_id is required")
	}

	limit := req.GetLimit()
	if limit <= 0 {
		limit = 50 // Default history fetch size
	}

	var beforeTime time.Time
	if req.GetBeforeTimestamp() != nil {
		beforeTime = req.GetBeforeTimestamp().AsTime()
	}

	dbMsgs, err := s.db.GetMessagesForRoom(ctx, roomID, int(limit), beforeTime)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to query messages: %v", err)
	}

	protoMsgs := make([]*chat.Message, 0, len(dbMsgs))
	for _, m := range dbMsgs {
		protoMsgs = append(protoMsgs, &chat.Message{
			Id:          m.ID,
			RoomId:      m.RoomID,
			SenderId:    m.SenderID,
			Content:     m.Content,
			MessageType: messageTypeToProto(m.MessageType),
			CreatedAt:   timestamppb.New(m.CreatedAt),
		})
	}

	return &chat.GetMessagesResponse{Messages: protoMsgs}, nil
}

func (s *ChatServer) GetRoomMembers(ctx context.Context, req *chat.GetRoomMembersRequest) (*chat.GetRoomMembersResponse, error) {
	roomID := req.GetRoomId()
	if roomID == "" {
		return nil, status.Error(codes.InvalidArgument, "room_id is required")
	}

	dbMembers, err := s.db.GetRoomMembers(ctx, roomID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to query room members: %v", err)
	}

	protoMembers := make([]*chat.RoomMember, 0, len(dbMembers))
	for _, m := range dbMembers {
		protoMembers = append(protoMembers, &chat.RoomMember{
			RoomId:      m.RoomID,
			UserId:      m.UserID,
			JoinedAt:    timestamppb.New(m.JoinedAt),
			DisplayName: m.DisplayName,
			Username:    m.Username,
		})
	}

	return &chat.GetRoomMembersResponse{Members: protoMembers}, nil
}

func (s *ChatServer) UploadFile(stream chat.ChatService_UploadFileServer) error {
	var file *os.File
	var filename string
	var totalBytes int64

	defer func() {
		if file != nil {
			file.Close()
		}
	}()

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to receive chunk: %v", err)
		}

		if file == nil {
			filename = chunk.GetFilename()
			if filename == "" {
				return status.Error(codes.InvalidArgument, "filename is required in first chunk")
			}

			// Sanitize filename to avoid directory traversal
			filename = filepath.Base(filename)
			// Ensure unique filename
			filename = fmt.Sprintf("%d-%s", time.Now().UnixNano(), filename)

			uploadDir := "/app/uploads"
			if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
				return status.Errorf(codes.Internal, "failed to create upload directory: %v", err)
			}

			filePath := filepath.Join(uploadDir, filename)
			file, err = os.Create(filePath)
			if err != nil {
				return status.Errorf(codes.Internal, "failed to create file: %v", err)
			}
		}

		n, err := file.Write(chunk.GetChunk())
		if err != nil {
			return status.Errorf(codes.Internal, "failed to write chunk: %v", err)
		}
		totalBytes += int64(n)
	}

	if file != nil {
		file.Close()
		file = nil
	}

	if totalBytes == 0 {
		return status.Error(codes.InvalidArgument, "uploaded file is empty")
	}

	fileURL := fmt.Sprintf("/uploads/%s", filename)
	return stream.SendAndClose(&chat.UploadFileResponse{
		FilePath: fileURL,
	})
}

