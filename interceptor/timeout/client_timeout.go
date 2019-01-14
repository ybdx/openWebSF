package timeout

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

// set request timeout, default value is 6000ms
func UnaryTimeOut(timeout time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		newCtx := ctx
		if _, ok := ctx.Deadline(); !ok && timeout > 0 {
			var cancelFuc context.CancelFunc
			newCtx, cancelFuc = context.WithTimeout(newCtx, timeout)
			defer cancelFuc()
		}
		return invoker(newCtx, method, req, reply, cc, opts...)
	}
}
