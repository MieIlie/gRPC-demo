package server

import (
	"context"
	"time"

	"auth-service/internal/db"
	"auth-service/internal/utils"
	"gen/auth"
	"shared/logger"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	auth.UnimplementedAuthServiceServer
	db        *db.DB
	jwtSecret string
}

func NewServer(database *db.DB, secret string) *Server {
	return &Server{
		db:        database,
		jwtSecret: secret,
	}
}

func (s *Server) Register(ctx context.Context, req *auth.RegisterRequest) (*auth.RegisterResponse, error) {
	if req.Username == "" || req.Password == "" || req.DisplayName == "" {
		return nil, status.Error(codes.InvalidArgument, "username, password, and display name are required")
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		logger.Error("Failed to hash password: %v", err)
		return nil, status.Error(codes.Internal, "failed to process request")
	}

	user, err := s.db.CreateUser(ctx, req.Username, hashedPassword, req.DisplayName, req.AvatarUrl)
	if err != nil {
		logger.Error("Failed to create user in database: %v", err)
		return nil, status.Error(codes.AlreadyExists, "username already taken")
	}

	token, refreshToken, err := utils.GenerateTokenPair(user.ID, user.Username, s.jwtSecret)
	if err != nil {
		logger.Error("Failed to generate token pair: %v", err)
		return nil, status.Error(codes.Internal, "failed to generate session")
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	err = s.db.CreateSession(ctx, user.ID, refreshToken, expiresAt)
	if err != nil {
		logger.Error("Failed to store refresh token session: %v", err)
		return nil, status.Error(codes.Internal, "failed to save session")
	}

	return &auth.RegisterResponse{
		UserId:       user.ID,
		Username:     user.Username,
		DisplayName:  user.DisplayName,
		Token:        token,
		RefreshToken: refreshToken,
	}, nil
}

func (s *Server) Login(ctx context.Context, req *auth.LoginRequest) (*auth.LoginResponse, error) {
	if req.Username == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "username and password are required")
	}

	user, err := s.db.GetUserByUsername(ctx, req.Username)
	if err != nil {
		logger.Error("User not found or database error for user %s: %v", req.Username, err)
		return nil, status.Error(codes.Unauthenticated, "invalid username or password")
	}

	if !utils.CheckPasswordHash(req.Password, user.PasswordHash) {
		return nil, status.Error(codes.Unauthenticated, "invalid username or password")
	}

	token, refreshToken, err := utils.GenerateTokenPair(user.ID, user.Username, s.jwtSecret)
	if err != nil {
		logger.Error("Failed to generate token pair: %v", err)
		return nil, status.Error(codes.Internal, "failed to generate session")
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	err = s.db.CreateSession(ctx, user.ID, refreshToken, expiresAt)
	if err != nil {
		logger.Error("Failed to store refresh token session: %v", err)
		return nil, status.Error(codes.Internal, "failed to save session")
	}

	return &auth.LoginResponse{
		UserId:       user.ID,
		Username:     user.Username,
		DisplayName:  user.DisplayName,
		Token:        token,
		RefreshToken: refreshToken,
	}, nil
}

func (s *Server) ValidateToken(ctx context.Context, req *auth.ValidateTokenRequest) (*auth.ValidateTokenResponse, error) {
	if req.Token == "" {
		return &auth.ValidateTokenResponse{Valid: false}, nil
	}

	claims, err := utils.ValidateToken(req.Token, s.jwtSecret)
	if err != nil {
		logger.Info("Token validation failed: %v", err)
		return &auth.ValidateTokenResponse{Valid: false}, nil
	}

	return &auth.ValidateTokenResponse{
		Valid:    true,
		UserId:   claims.UserID,
		Username: claims.Username,
	}, nil
}
