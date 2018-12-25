package service

import (
	"openWebSF/example/pb"
	"context"
)

type HelloServer struct{}

func (s *HelloServer) HelloWorld(ctx context.Context, in *pb.HelloRequest) (*pb.HelloRespone, error) {
	return &pb.HelloRespone{Name: "hello " + in.Name}, nil
}
