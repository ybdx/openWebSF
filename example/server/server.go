package main

import (
	"flag"
	"openWebSF/example/pb"
	"openWebSF/example/server/service"
	"openWebSF/server"
)

var (
	port       = flag.Int("port", 9301, "port")
	label      = flag.String("label", "default", "")
	weight     = flag.String("weight", "50", "")
	service1    = flag.String("service", "wosf.hello.v1.helloService", "service name")
	serverType = flag.String("type", "simple", "simple | shutdown | interceptor | limitQPS | reflection")
)

func main() {
	flag.Parse()
	switch *serverType {
	case "shutdown":
		return
	default:
		simpleServer()
	}
}

func simpleServer() {
	helloService := server.ServiceConfig{
		Name:    *service1,
		RegisterService: pb.RegisterHelloServiceServer,
		Server:   &service.HelloServer{},
	}
	server.NewServer().
		Register(helloService).
		Start(*port)
}