package client

import (
	"context"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/naming"
	"openWebSF/balancer/random"
	"openWebSF/balancer/roundrobin"
	"openWebSF/config"
	"openWebSF/interceptor/monitor"
	"openWebSF/interceptor/pass_metadata"
	client_timeout "openWebSF/interceptor/timeout"
	"openWebSF/registry"
	"openWebSF/resolver"
	"os"
	"sync"
	"time"
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

const (
	DefaultReqTimeout       = 6000
	DefaultMonitorThreshold = 10
)

type ClientConfig struct {
	Service          string            // 服务名， 不为空的时候通过服务名发现服务
	Registry         string            // zk或其它注册中心地址，使用直连方式时此字段为空
	DirectAddr       map[string]string // Service字段为空时需要设置直接的地址
	Balancer         Balancer          // 负载均衡器，不设置则使用默认的,默认值为WRoundRobin, 使用expreimental相关的接口的时候必须设置
	Experimental     bool              // 是否是采用grpc expreimental相关的接口 false表示不是
	dialOpts         []grpc.DialOption
	StreamInt        grpc.StreamClientInterceptor // 设置interceptor
	UnaryInt         grpc.UnaryClientInterceptor
	ReqTimeout       int // 请求超时，单位 ms，默认 6000 ms
	MonitorThreshold int // 打印 monitor 日志的阈值，单位 ms，默认 10 ms
}

var register = struct {
	sync.RWMutex
	r *registry.Registry
}{}

func experimentInit(conf ClientConfig) string {
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

	var name string
	switch conf.Balancer {
	case WRoundRobinExperimental:
		name = roundrobin.Init(true)
	case RoundRobinExperimental:
		name = roundrobin.Init(false)
	case RandomExperimental:
		name = random.Init(false)
	case WRandomExperimental:
		name = random.Init(true)
	default:
		logrus.Fatalln("NewClient() parameter invalid, unsupported balancer type")
	}
	return name
}

func originInit(conf ClientConfig) (grpc.Balancer, error) {
	var r naming.Resolver
	switch {
	case conf.Service != "":
		r = resolver.ZookeeperResolve(conf.Service)
		if conf.Registry == "" {
			logrus.Fatalln("NewClient must have specify ClientConfig.Registry")
		}
	default:
		logrus.Fatalln("NewClient() parameter invalid, must set ClientConfig.Server or ClientConfig.DirectIP")
	}

	var b grpc.Balancer
	switch conf.Balancer {
	case RoundRobin:
		b = roundrobin.RoundRobin(r, false)
	case WRoundRobin:
		b = roundrobin.RoundRobin(r, true)
	case Random:
		b = random.Random(r, false)
	case WRandom:
		b = random.Random(r, true)
	default:
		logrus.Fatalln("NewClient() parameter invalid, unsupported balancer type")
	}

	return b, nil
}

func NewClient(conf ClientConfig) *grpc.ClientConn {

	conf.dialOpts = []grpc.DialOption{
		grpc.WithInsecure(),
	}
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)

	if conf.Experimental {
		name := experimentInit(conf)
		conf.dialOpts = append(conf.dialOpts, grpc.WithBalancerName(name))
	} else {
		b, _ := originInit(conf)
		conf.dialOpts = append(conf.dialOpts, grpc.WithBalancer(b))
	}

	conf.passTraceId()
	conf.setReqTimeout()
	conf.setMonitorLog()

	// after all interceptor is set, then use this function
	conf.addInterceptorBeforeDial()

	conn, err := grpc.DialContext(ctx, conf.Registry, conf.dialOpts...)

	if err != nil {
		logrus.Fatalf("grpc.DialContext failed, service[%s]  error:%s", conf.Service, err)
	}
	if conf.Service != "" {
		if nil == register.r {
			register.Lock()
			if r := registry.Register(conf.Registry); r == nil {
				logrus.Fatalf("client create registry connection failed")
			} else {
				register.r = r
			}
			register.Unlock()
		}
		if err := register.r.RegisterClient(conf.Service, os.Getegid()); err != nil {
			logrus.Warnf("register client to registration center failed. %s", err)
		}
	}
	return conn
}

// set request timeout, default value is 6000ms
func (c *ClientConfig) setReqTimeout() {
	timeout := DefaultReqTimeout * time.Millisecond
	if c.ReqTimeout > 0 {
		timeout = time.Duration(c.ReqTimeout) * time.Millisecond
	}
	c.AddUnaryInterceptor(client_timeout.UnaryTimeOut(timeout))
}

func (c *ClientConfig) setMonitorLog() {
	monitorThreshold := DefaultMonitorThreshold * time.Millisecond
	if c.MonitorThreshold > 0 {
		monitorThreshold = time.Duration(c.MonitorThreshold) * time.Millisecond
	}
	c.AddUnaryInterceptor(monitor.UnaryMonitorLog(monitorThreshold))
	c.AddStreamInterceptor(monitor.StreamMonitorLog(monitorThreshold))
	c.AddStreamInterceptor()
}

// pass traceId
func (c *ClientConfig) passTraceId() {
	c.AddStreamInterceptor(pass_metadata.StreamPass(config.TraceIdKey))
	c.AddUnaryInterceptor(pass_metadata.UnaryPass(config.TraceIdKey))
}

func (c *ClientConfig) addInterceptorBeforeDial() {
	if c.UnaryInt != nil {
		c.dialOpts = append(c.dialOpts, grpc.WithUnaryInterceptor(c.UnaryInt))
	}
	if c.StreamInt != nil {
		c.dialOpts = append(c.dialOpts, grpc.WithStreamInterceptor(c.StreamInt))
	}
}

// add stream interceptor
func (c *ClientConfig) AddStreamInterceptor(interceptor ...grpc.StreamClientInterceptor) *ClientConfig {
	interceptors := make([]grpc.StreamClientInterceptor, 0)
	if c.StreamInt != nil {
		interceptors = append(interceptors, c.StreamInt)
	}
	interceptors = append(interceptors, interceptor...)
	c.StreamInt = grpc_middleware.ChainStreamClient(interceptors...)
	return c
}

// add unary interceptor
func (c *ClientConfig) AddUnaryInterceptor(interceptor ...grpc.UnaryClientInterceptor) *ClientConfig {
	interceptors := make([]grpc.UnaryClientInterceptor, 0)
	if c.UnaryInt != nil {
		interceptors = append(interceptors, c.UnaryInt)
	}
	interceptors = append(interceptors, interceptor...)
	c.UnaryInt = grpc_middleware.ChainUnaryClient(interceptors...)
	return c
}
