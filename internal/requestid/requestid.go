package requestid

import (
	"context"
)

type RequestIDKey string

const Key RequestIDKey = "requestID"

func Get(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(Key).(string)
	return val, ok
}