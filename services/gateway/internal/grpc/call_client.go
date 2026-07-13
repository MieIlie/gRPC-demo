package grpc

import (
	"context"
	"fmt"
	"gen/call"
	"shared/logger"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type CallClient struct {
	client call.CallServiceClient
	conn   *grpc.ClientConn
}

func NewCallClient(addr string) (*CallClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize call service client at %s: %w", addr, err)
	}

	logger.Info("Initialized Call Service client targeting %s", addr)
	client := call.NewCallServiceClient(conn)

	return &CallClient{
		client: client,
		conn:   conn,
	}, nil
}

func (cc *CallClient) Close() error {
	if cc.conn != nil {
		return cc.conn.Close()
	}
	return nil
}

func (cc *CallClient) StartCall(ctx context.Context, roomID, callerID, receiverID string, callType call.CallType) (*call.StartCallResponse, error) {
	return cc.client.StartCall(ctx, &call.StartCallRequest{
		RoomId:     roomID,
		CallerId:   callerID,
		ReceiverId: receiverID,
		CallType:   callType,
	})
}

func (cc *CallClient) AcceptCall(ctx context.Context, callID string) (*call.AcceptCallResponse, error) {
	return cc.client.AcceptCall(ctx, &call.AcceptCallRequest{
		CallId: callID,
	})
}

func (cc *CallClient) RejectCall(ctx context.Context, callID string) (*call.RejectCallResponse, error) {
	return cc.client.RejectCall(ctx, &call.RejectCallRequest{
		CallId: callID,
	})
}

func (cc *CallClient) EndCall(ctx context.Context, callID string) (*call.EndCallResponse, error) {
	return cc.client.EndCall(ctx, &call.EndCallRequest{
		CallId: callID,
	})
}

func (cc *CallClient) GetCallSession(ctx context.Context, callID string) (*call.GetCallSessionResponse, error) {
	return cc.client.GetCallSession(ctx, &call.GetCallSessionRequest{
		CallId: callID,
	})
}
