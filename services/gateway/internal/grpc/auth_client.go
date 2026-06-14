package grpc

import (
	"context"
	"fmt"
	"time"

	"gen/auth"
	"gateway/internal/trace"
	"shared/logger"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AuthClient struct {
	client auth.AuthServiceClient
	conn   *grpc.ClientConn
}

func NewAuthClient(addr string) (*AuthClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize auth service client at %s: %w", addr, err)
	}

	logger.Info("Initialized Auth Service client targeting %s", addr)
	client := auth.NewAuthServiceClient(conn)

	return &AuthClient{
		client: client,
		conn:   conn,
	}, nil
}

func (ac *AuthClient) Close() error {
	if ac.conn != nil {
		return ac.conn.Close()
	}
	return nil
}

func (ac *AuthClient) Register(ctx context.Context, username, password, displayName, avatarURL string) (*auth.RegisterResponse, error) {
	start := time.Now()
	resp, err := ac.client.Register(ctx, &auth.RegisterRequest{
		Username:    username,
		Password:    password,
		DisplayName: displayName,
		AvatarUrl:   avatarURL,
	})
	duration := time.Since(start).Milliseconds()

	status := "success"
	msgText := fmt.Sprintf("RegisterUser { username: %s, displayName: %s }", username, displayName)
	if err != nil {
		status = "error"
		msgText = fmt.Sprintf("RegisterUser error: %v", err)
	}

	trace.GetTracker().Record(&trace.Event{
		Source:     "Gateway",
		Target:     "Auth Service",
		Protocol:   "gRPC",
		Type:       "Request/Response",
		Message:    msgText,
		Status:     status,
		DurationMs: duration,
	})

	return resp, err
}

func (ac *AuthClient) Login(ctx context.Context, username, password string) (*auth.LoginResponse, error) {
	start := time.Now()
	resp, err := ac.client.Login(ctx, &auth.LoginRequest{
		Username: username,
		Password: password,
	})
	duration := time.Since(start).Milliseconds()

	status := "success"
	msgText := fmt.Sprintf("LoginUser { username: %s }", username)
	if err != nil {
		status = "error"
		msgText = fmt.Sprintf("LoginUser error: %v", err)
	}

	trace.GetTracker().Record(&trace.Event{
		Source:     "Gateway",
		Target:     "Auth Service",
		Protocol:   "gRPC",
		Type:       "Request/Response",
		Message:    msgText,
		Status:     status,
		DurationMs: duration,
	})

	return resp, err
}

func (ac *AuthClient) ValidateToken(ctx context.Context, token string) (*auth.ValidateTokenResponse, error) {
	start := time.Now()
	resp, err := ac.client.ValidateToken(ctx, &auth.ValidateTokenRequest{
		Token: token,
	})
	duration := time.Since(start).Milliseconds()

	status := "success"
	msgText := "ValidateToken"
	if err != nil {
		status = "error"
		msgText = fmt.Sprintf("ValidateToken error: %v", err)
	} else if resp != nil {
		msgText = fmt.Sprintf("ValidateToken { user_id: %s, username: %s }", resp.UserId, resp.Username)
	}

	trace.GetTracker().Record(&trace.Event{
		Source:     "Gateway",
		Target:     "Auth Service",
		Protocol:   "gRPC",
		Type:       "Request/Response",
		Message:    msgText,
		Status:     status,
		DurationMs: duration,
	})

	return resp, err
}
