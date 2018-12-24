package main

import (
	"flag"
	"time"
	"fmt"
	"context"
	"openWebSF/example/pb"
	"openWebSF/client"
)

var (
	clientType     = flag.String("type", "simple", "simple | long_time_call | infinite | header | crazy | reflection")
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
		Service:  "ofo.user.v1.userService",
		Registry: *clientRegistry,
		Balancer: client.RoundRobinExperimental,
		Experimental: true,
	})
	ticker := time.NewTicker(1000 * time.Millisecond)

	client := pb.NewUserServiceClient(conn)
	resp, err := client.QueryUserByTel(context.Background(), &pb.QueryUserByTelRequest{Tel: "13880678489"})
	if err == nil {
		fmt.Printf("Reply is %s\n", resp.Name)
	} else {
		fmt.Println("err:", err)
	}
	for t := range ticker.C {
		resp, err := client.QueryUserByTel(context.Background(), &pb.QueryUserByTelRequest{Tel: "13880678489"})
		if err == nil {
			fmt.Printf("%v: Reply is %s\n", t, resp.Name)
		} else {
			fmt.Println("err:", err)
		}
	}
}