package interceptors

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

    "github.com/google/uuid"

    "github.com/kay-kewl/ticket-booking-system/internal/requestid"
)

const requestIDMetadataKey = "x-request-id"

func ClientRequestIDInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if requestID, ok := requestid.Get(ctx); ok {
			ctx = metadata.AppendToOutgoingContext(ctx, requestIDMetadataKey, requestID)
		}

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func ServerRequestIDInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
        var requestID string
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get(requestIDMetadataKey); len(values) > 0 {
                requestID = values[0]
            }
		}

        if requestID == "" {
            requestID = uuid.NewString()
        }

        ctx = context.WithValue(ctx, requestid.Key, requestID)

		return handler(ctx, req)
	}
}
