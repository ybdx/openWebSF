package main

import (
	"flag"
	"time"
	"fmt"
	"context"
	"openWebSF/example/pb"
	"openWebSF/client"
	"google.golang.org/grpc/metadata"
)

var (
	clientType     = flag.String("type", "simple", "simple")
	clientRegistry = flag.String("registry", "zookeeper:///10.2.40.71:2181,10.2.40.93:2181,10.2.40.99:2181", "")
	clientNum      = flag.Int("n", 10, "the number of concurrent request")
)

func main() {
	flag.Parse()
	switch *clientType {
	case "long_time_call":
		return
	default:
		simpleClient()
	}
}

func simpleClient() {
	conn := client.NewClient(client.ClientConfig{
		Service:"wosf.hello.v1.helloService",
		Registry: *clientRegistry,
		Balancer: client.RoundRobinExperimental,
		Experimental: true,
	})
	clientU := pb.NewHelloServiceClient(conn)

	ticker := time.NewTicker(1000 * time.Millisecond)
	md := make(map[string]string)
	md["trace"] = "ybdx"


	for t := range ticker.C {
		resp, err := clientU.HelloWorld(metadata.NewIncomingContext(context.Background(),metadata.Pairs("trace_id", "ybdx", "trace_id", "test")),
			&pb.HelloRequest{Name: "ybdx vs you"})
		if err == nil {
			fmt.Printf("%v: Reply is %s\n", t, resp.Name)
		} else {
			fmt.Println("err:", err)
		}
	}
}

