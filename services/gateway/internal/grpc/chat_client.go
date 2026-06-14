package grpc

import (
	"context"
	"fmt"
	"gen/chat"
	"shared/logger"

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
