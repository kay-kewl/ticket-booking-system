package grpc

import (
	"context"

	"google.golang.org/grpc"

	authv1 "github.com/kay-kewl/ticket-booking-system/gen/go/auth"
)

type Auth interface {
	Login(ctx context.Context, email string, password string) (token string, err error)
	Register(ctx context.Context, email string, password string) (userID int64, err error)
}

type serverAPI struct {
	authv1.UnimplementedAuthServer
	auth Auth
}

func Register(gRPCServer *grpc.Server, auth Auth) {
	authv1.RegisterAuthServer(gRPCServer, &serverAPI{auth: auth})
}

func (s *serverAPI) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	userID, err := s.auth.Register(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		// TODO: handle errors
		return nil, error
	}

	return &authv1.RegisterResponse{UserId: userID}, nil
}