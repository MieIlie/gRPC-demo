package server

import (
	"context"
	"gen/call"
	"call-service/internal/db"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CallServer struct {
	call.UnimplementedCallServiceServer
	db *db.DB
}

func NewCallServer(database *db.DB) *CallServer {
	return &CallServer{db: database}
}

func callTypeToDB(ct call.CallType) string {
	if ct == call.CallType_VIDEO {
		return "video"
	}
	return "voice"
}

func callTypeToProto(ct string) call.CallType {
	if ct == "video" {
		return call.CallType_VIDEO
	}
	return call.CallType_VOICE
}

func callStatusToProto(cs string) call.CallStatus {
	switch cs {
	case "accepted":
		return call.CallStatus_ACCEPTED
	case "rejected":
		return call.CallStatus_REJECTED
	case "ended":
		return call.CallStatus_ENDED
	case "missed":
		return call.CallStatus_MISSED
	default:
		return call.CallStatus_PENDING
	}
}

func dbSessionToProto(s *db.CallSession) *call.CallSession {
	var roomID string
	if s.RoomID.Valid {
		roomID = s.RoomID.String
	}

	var startedAt *timestamppb.Timestamp
	if s.StartedAt.Valid {
		startedAt = timestamppb.New(s.StartedAt.Time)
	}

	var endedAt *timestamppb.Timestamp
	if s.EndedAt.Valid {
		endedAt = timestamppb.New(s.EndedAt.Time)
	}

	return &call.CallSession{
		Id:         s.ID,
		RoomId:     roomID,
		CallerId:   s.CallerID,
		ReceiverId: s.ReceiverID,
		CallType:   callTypeToProto(s.CallType),
		Status:     callStatusToProto(s.Status),
		StartedAt:  startedAt,
		EndedAt:    endedAt,
		CreatedAt:  timestamppb.New(s.CreatedAt),
	}
}

func (s *CallServer) StartCall(ctx context.Context, req *call.StartCallRequest) (*call.StartCallResponse, error) {
	callerID := req.GetCallerId()
	receiverID := req.GetReceiverId()
	roomID := req.GetRoomId()
	if callerID == "" || receiverID == "" {
		return nil, status.Error(codes.InvalidArgument, "caller_id and receiver_id are required")
	}

	dbSession, err := s.db.StartCall(ctx, roomID, callerID, receiverID, callTypeToDB(req.GetCallType()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to start call: %v", err)
	}

	return &call.StartCallResponse{Session: dbSessionToProto(dbSession)}, nil
}

func (s *CallServer) AcceptCall(ctx context.Context, req *call.AcceptCallRequest) (*call.AcceptCallResponse, error) {
	callID := req.GetCallId()
	if callID == "" {
		return nil, status.Error(codes.InvalidArgument, "call_id is required")
	}

	dbSession, err := s.db.AcceptCall(ctx, callID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to accept call: %v", err)
	}

	return &call.AcceptCallResponse{Session: dbSessionToProto(dbSession)}, nil
}

func (s *CallServer) RejectCall(ctx context.Context, req *call.RejectCallRequest) (*call.RejectCallResponse, error) {
	callID := req.GetCallId()
	if callID == "" {
		return nil, status.Error(codes.InvalidArgument, "call_id is required")
	}

	dbSession, err := s.db.RejectCall(ctx, callID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to reject call: %v", err)
	}

	return &call.RejectCallResponse{Session: dbSessionToProto(dbSession)}, nil
}

func (s *CallServer) EndCall(ctx context.Context, req *call.EndCallRequest) (*call.EndCallResponse, error) {
	callID := req.GetCallId()
	if callID == "" {
		return nil, status.Error(codes.InvalidArgument, "call_id is required")
	}

	dbSession, err := s.db.EndCall(ctx, callID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to end call: %v", err)
	}

	return &call.EndCallResponse{Session: dbSessionToProto(dbSession)}, nil
}

func (s *CallServer) GetCallSession(ctx context.Context, req *call.GetCallSessionRequest) (*call.GetCallSessionResponse, error) {
	callID := req.GetCallId()
	if callID == "" {
		return nil, status.Error(codes.InvalidArgument, "call_id is required")
	}

	dbSession, err := s.db.GetCallSession(ctx, callID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to query call session: %v", err)
	}
	if dbSession == nil {
		return nil, status.Error(codes.NotFound, "call session not found")
	}

	return &call.GetCallSessionResponse{Session: dbSessionToProto(dbSession)}, nil
}
