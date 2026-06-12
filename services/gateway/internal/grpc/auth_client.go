package grpc

import (
	"context"
	"fmt"
	"time"

	"gen/auth"
	"shared/logger"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AuthClient struct {
	client auth.AuthServiceClient
	conn   *grpc.ClientConn
}

func NewAuthClient(addr string) (*AuthClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return nil, fmt.Errorf("failed to dial auth service at %s: %w", addr, err)
	}

	logger.Info("Connected to Auth Service gRPC at %s", addr)
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
	return ac.client.Register(ctx, &auth.RegisterRequest{
		Username:    username,
		Password:    password,
		DisplayName: displayName,
		AvatarUrl:   avatarURL,
	})
}

func (ac *AuthClient) Login(ctx context.Context, username, password string) (*auth.LoginResponse, error) {
	return ac.client.Login(ctx, &auth.LoginRequest{
		Username: username,
		Password: password,
	})
}

func (ac *AuthClient) ValidateToken(ctx context.Context, token string) (*auth.ValidateTokenResponse, error) {
	return ac.client.ValidateToken(ctx, &auth.ValidateTokenRequest{
		Token: token,
	})
}
