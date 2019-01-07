package service

import (
	"openWebSF/example/pb"
	"context"
	"fmt"
)

type HelloServer struct{}

func (s *HelloServer) HelloWorld(ctx context.Context, in *pb.HelloRequest) (*pb.HelloRespone, error) {
	fmt.Println(ctx.Value("traceId"), ctx)
	return &pb.HelloRespone{Name: "hello " + in.Name}, nil
}
