package pass_metadata

import (
	"google.golang.org/grpc"
	"context"
	"google.golang.org/grpc/metadata"
)

func StreamPass(keys ...string) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(newOutIncomingMetadata(ctx, keys...), desc, cc, method, opts...)
	}
}

func UnaryPass(keys ...string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(newOutIncomingMetadata(ctx, keys...), method, req, reply, cc, opts...)
	}
}

// 将 incoming 中的指定的 metadata 复制到 outgoing 中
func newOutIncomingMetadata(ctx context.Context, keys ...string) context.Context {
	newCtx := ctx
	if inMD, ok := metadata.FromIncomingContext(ctx); ok {
		for _, key := range keys {
			if len(inMD[key]) > 0 {
				for _, v := range inMD[key] {
					newCtx = metadata.AppendToOutgoingContext(newCtx, key, v)
				}
			}
		}
	}
	return newCtx
}