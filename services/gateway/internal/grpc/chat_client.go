package grpc

import (
	"context"
	"fmt"
	"gen/chat"
	"gateway/internal/trace"
	"io"
	"shared/logger"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ChatClient struct {
	client chat.ChatServiceClient
	conn   *grpc.ClientConn
}

func NewChatClient(addr string) (*ChatClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize chat service client at %s: %w", addr, err)
	}

	logger.Info("Initialized Chat Service client targeting %s", addr)
	client := chat.NewChatServiceClient(conn)

	return &ChatClient{
		client: client,
		conn:   conn,
	}, nil
}

func (cc *ChatClient) Close() error {
	if cc.conn != nil {
		return cc.conn.Close()
	}
	return nil
}

func (cc *ChatClient) CreateRoom(ctx context.Context, roomType chat.RoomType, roomName, createdBy string, memberIDs []string) (*chat.CreateRoomResponse, error) {
	return cc.client.CreateRoom(ctx, &chat.CreateRoomRequest{
		RoomType:  roomType,
		RoomName:  roomName,
		CreatedBy: createdBy,
		MemberIds: memberIDs,
	})
}

func (cc *ChatClient) GetRooms(ctx context.Context, userID string) (*chat.GetRoomsResponse, error) {
	return cc.client.GetRooms(ctx, &chat.GetRoomsRequest{
		UserId: userID,
	})
}

func (cc *ChatClient) SendMessage(ctx context.Context, roomID, senderID, content string, msgType chat.MessageType) (*chat.SendMessageResponse, error) {
	return cc.client.SendMessage(ctx, &chat.SendMessageRequest{
		RoomId:      roomID,
		SenderId:    senderID,
		Content:     content,
		MessageType: msgType,
	})
}

func (cc *ChatClient) GetMessages(ctx context.Context, roomID string, limit int32, beforeTime *timestamppb.Timestamp) (*chat.GetMessagesResponse, error) {
	return cc.client.GetMessages(ctx, &chat.GetMessagesRequest{
		RoomId:          roomID,
		Limit:           limit,
		BeforeTimestamp: beforeTime,
	})
}

func (cc *ChatClient) GetRoomMembers(ctx context.Context, roomID string) (*chat.GetRoomMembersResponse, error) {
	return cc.client.GetRoomMembers(ctx, &chat.GetRoomMembersRequest{
		RoomId: roomID,
	})
}

func (cc *ChatClient) UploadFile(ctx context.Context, filename string, r io.Reader) (string, error) {
	start := time.Now()
	stream, err := cc.client.UploadFile(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to open upload stream: %w", err)
	}

	buf := make([]byte, 32*1024) // 32KB chunk size
	var totalBytes int64
	for {
		n, err := r.Read(buf)
		if n > 0 {
			errSend := stream.Send(&chat.UploadFileChunk{
				Chunk:    buf[:n],
				Filename: filename,
			})
			if errSend != nil {
				return "", fmt.Errorf("failed to send chunk: %w", errSend)
			}
			totalBytes += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read input file: %w", err)
		}
	}

	resp, err := stream.CloseAndRecv()
	duration := time.Since(start).Milliseconds()

	statusVal := "success"
	var msgText string
	if err != nil {
		statusVal = "error"
		msgText = fmt.Sprintf("UploadFile error: %v", err)
	} else {
		sec := float64(duration) / 1000.0
		mb := float64(totalBytes) / (1024 * 1024)
		speedStr := ""
		if sec > 0 {
			speedStr = fmt.Sprintf(" (%.2f MB/s)", mb/sec)
		}
		msgText = fmt.Sprintf("UploadFile { filename: %s, size: %.2f MB%s }", filename, mb, speedStr)
	}

	trace.GetTracker().Record(&trace.Event{
		Source:     "Gateway",
		Target:     "Chat Service",
		Protocol:   "gRPC",
		Type:       "Streaming",
		Message:    msgText,
		Status:     statusVal,
		DurationMs: duration,
	})

	if err != nil {
		return "", err
	}
	return resp.GetFilePath(), nil
}

