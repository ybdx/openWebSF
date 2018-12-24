package client

import (
	"google.golang.org/grpc"
	"openWebSF/balancer/roundrobin"
	"openWebSF/resolver"
	"context"
	"time"
	"github.com/sirupsen/logrus"
	"fmt"
)

type Balancer uint8

const (
	WRoundRobin Balancer = iota // weighted round robin
	RoundRobin                  // round robin
	Random                      // random
	WRandom                     // weighted random

	// experimentall param
	WRoundRobinExperimental
	RoundRobinExperimental
	RandomExperimental
	WRandomExperimental
)

type ClientConfig struct {
	Service      string            // 服务名， 不为空的时候通过服务名发现服务
	Registry     string            // zk或其它注册中心地址，使用直连方式时此字段为空
	DirectAddr   map[string]string // Service字段为空时需要设置直接的地址
	Balancer     Balancer          // 负载均衡器，不设置则使用默认的,默认值为WRoundRobin, 使用expreimental相关的接口的时候必须设置
	Experimental bool              // 是否是采用grpc expreimental相关的接口 false表示不是
	dialOpts     []grpc.DialOption
}

func experimentInit(conf ClientConfig) {
	switch {
	case conf.Service != "":
		resolver.Init(conf.Service)
		if conf.Registry == "" {
			logrus.Fatalln("NewClient must specify ClientConfig.Registry")
		}
	case len(conf.DirectAddr) > 0:
	default:
		logrus.Fatalln("NewClient() parameter invalid, must set ClientConfig.Server or ClientConfig.DirectIP")
	}
	fmt.Println(conf.Balancer)

	switch conf.Balancer {
	case WRoundRobinExperimental:
	case RoundRobinExperimental:
		roundrobin.Init(false)
	case RandomExperimental:
	case WRandomExperimental:
	default:
		logrus.Fatalln("NewClient() parameter invalid, unsupported balancer type")
	}

}

// todo
func originInit(conf ClientConfig) (grpc.Balancer, error) {
	return nil, nil
}

func NewClient(conf ClientConfig) *grpc.ClientConn {
	if conf.Experimental {
		experimentInit(conf)
	} else {
		originInit(conf)
	}
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	conf.dialOpts = []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithBalancerName(roundrobin.Name),
	}
	conn, err := grpc.DialContext(ctx, conf.Registry, conf.dialOpts...)

	if err != nil {
		logrus.Fatalf("grpc.DialContext failed, service[%s]  error:%s", conf.Service, err)
	}
	return conn
}
