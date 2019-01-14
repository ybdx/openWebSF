package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

const logTimePattern = "2006-01-02 15:04:05.000"

var logger monitorLogger

// 创建parent interface, log.Logger实现了该接口
type monitorLogger interface {
	Printf(format string, v ...interface{})
}

func SetMonitorLog(l monitorLogger) {
	logger = l
}

func UnaryMonitorLog(threshold time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		startTime := time.Now()
		p := peer.Peer{}
		tempOpt := opts
		if tempOpt == nil {
			tempOpt = []grpc.CallOption{grpc.Peer(&p)}
		} else {
			tempOpt = append(tempOpt, grpc.Peer(&p))
		}
		err := invoker(ctx, method, req, reply, cc, tempOpt...)
		printMonitorLog(threshold, startTime, method, req, p.Addr)
		return err
	}
}

func StreamMonitorLog(threshold time.Duration) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		startTime := time.Now()
		p := peer.Peer{}
		tempOpt := opts
		if tempOpt == nil {
			tempOpt = []grpc.CallOption{grpc.Peer(&p)}
		} else {
			tempOpt = append(tempOpt, grpc.Peer(&p))
		}
		clientStream, err := streamer(ctx, desc, cc, method, tempOpt...)
		printMonitorLog(threshold, startTime, method, nil, p.Addr)
		return clientStream, err
	}
}

func printMonitorLog(threshold time.Duration, startTime time.Time, method string, req interface{}, peerAdr net.Addr) {
	cost := time.Since(startTime)
	if cost >= threshold {
		addr := "-"
		if peerAdr != nil {
			addr = peerAdr.String()
		}
		reqBytes := []byte("")
		if req != nil {
			reqBytes, _ = json.Marshal(req)
		}
		if len(reqBytes) > 2000 {
			reqBytes = reqBytes[:2000]
			msg := fmt.Sprintf("... total %d bytes", len(reqBytes))
			reqBytes = append(reqBytes, msg...)
		}
		costMs := cost / time.Millisecond
		logger.Printf("%s - - grpc %d ms grpc://%s%s %s\n", time.Now().Format(logTimePattern), costMs, addr, method, string(reqBytes))
	}
}
