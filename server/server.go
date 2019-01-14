package server

import (
	"google.golang.org/grpc"
	"openWebSF/config/server"
	"openWebSF/registry"
	"github.com/sirupsen/logrus"

	"reflect"
	"net"
	"strings"
	"strconv"
	"google.golang.org/grpc/reflection"
	"time"
	"openWebSF/config"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"sync"
)

type ServiceConfig struct {
	Name            string      // 服务名
	NoRegistration  bool        // 是否注册到注册中心， 默认值false,即注册到注册中心
	RegisterAddr    string      // 注册中心地址
	RegisterService interface{} // 生成的.pb.go文件中用于向grpc注册服务的函数，例如：RegisterPingServiceServer
	Server          interface{} // 调用Register时传入的第二个参数（实现.pb.go文件中Server interface的变量）
	metaInner       config.MetaDataInner
}

type server struct {
	server   *grpc.Server
	port     int // 注册端口号
	services map[string]ServiceConfig
	register *registry.Registry
	qpsChan  []chan int
}

func NewServer() *server {
	s := &server{
		services: make(map[string]ServiceConfig),
		qpsChan: make([]chan int, 2),  // qps限制
	}
	if serverConf.Conf.RegisterAddr != "" {
		s.register = registry.Register(serverConf.Conf.RegisterAddr)
		if s.register == nil {
			logrus.Fatalln("NewServer() initial register failed")
		}
	}
	return s
}

func (s *server) Register(service ServiceConfig) *server {
	if service.Name == "" {
		logrus.Fatalln("ServiceConfig.Name can't be empty")
	}
	if reflect.ValueOf(service.RegisterService).Kind() != reflect.Func {
		logrus.Fatalln("ServiceConfig.Register must be a function")
	}
	if reflect.ValueOf(service.Server).Kind() == reflect.Invalid {
		logrus.Fatalln("ServiceConfig.Server  invalid")
	}

	if !service.NoRegistration && s.register == nil {
		logrus.Fatalln("want register service to registration center, must specify the address in config file")
	}
	service.metaInner = config.DefaultMetaDataInner
	service.metaInner.Owner = serverConf.Conf.Owner
	if weight := os.Getenv(config.EnvServerWeight); weight != "" {
		w, err := strconv.Atoi(weight)
		if err != nil || w <= 0 {
			logrus.Warnf("SERVER_WEIGHT[%s] invalid, use default weight", weight)
		} else {
			service.metaInner.Weight = w
		}
	}
	s.services[service.Name] = service
	return s
}

func (s *server) Start(port int) {
	if serverConf.Conf.Zk.Servers != "" {
		defer s.register.Close()
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if nil != err {
		logrus.Fatalln(err)
	}

	s.port = port
	if 0 == s.port {
		addr := lis.Addr().String()
		items := strings.Split(addr, ":")
		if len(items) < 2 {
			logrus.Fatalf("use random port, but get real port failed. addr:[%s]", lis.Addr().String())
		}
		if p, err := strconv.Atoi(items[len(items)-1]); err != nil {
			logrus.Fatalf("use random port, but get real port failed. %s", err)
		} else {
			s.port = p
			logrus.Infof("server will use random port %d", s.port)
		}
	}

	s.server = grpc.NewServer()
	reflection.Register(s.server)
	for _, service := range s.services {
		f := reflect.ValueOf(service.RegisterService)
		in := []reflect.Value{
			reflect.ValueOf(s.server),
			reflect.ValueOf(service.Server),
		}
		f.Call(in)
	}

	go s.handleSignal()

	err = s.serveAndRegister(lis)

	if err == nil || strings.Contains(err.Error(), "use of closed network connection") {
		logrus.Infoln("owsf server shutdown success")
	} else {
		logrus.Errorln("Serve failed, error:", err)
	}
}

func (s *server) serveAndRegister(lis net.Listener) error {
	var err error
	go func() {
		time.Sleep(time.Second)
		if nil == err {
			for _, service := range s.services {
				if !service.NoRegistration {
					if nil == s.register {
						logrus.Fatalln("register is nil, this situation should not happen")
					}
					if err := s.register.RegisterService(service.Name, s.port, service.metaInner); err != nil {
						logrus.Warnf("register service [%s] to registration center failed, error: %v", service.Name, err)
					}
				}
			}
		}
	}()
	logrus.Infoln("starting serve request at port", s.port)
	err = s.server.Serve(lis)
	return err
}

func (s *server) handleSignal() {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGUSR1, syscall.SIGTERM)
	for c := range sigChan {
		switch c {
		case syscall.SIGINT, syscall.SIGTERM:
			s.Shutdown()
			return
		case syscall.SIGUSR1:
			s.reloadConfig()
		}
	}
}

func (s *server) reloadConfig() {
	newConfig := serverConf.GetConfFromFile()
	s.qpsChan[0] <- newConfig.LimitQPS
	s.qpsChan[1] <- newConfig.LimitQPS
}

func (s *server) Shutdown() {
	logrus.Infoln("shut down owsf server!!!")
	retry := 3

	unRegistryFailed := false
	var wg sync.WaitGroup
	for _, service := range s.services {
		if !service.NoRegistration {
			if nil == s.register {
				logrus.Errorln("register is nil, this situation should not be happen")
				continue
			}
			wg.Add(1)
			go func(conf ServiceConfig) {
				defer wg.Done()
				var err error
				for n:=1; n<= retry; n++ {
					if n > 1 {
						time.Sleep(time.Second * 3)
					}
					if err = s.register.UnRegisterService(conf.Name, s.port); err != nil {
						logrus.Warnf("unregister service[%s] failed(%d), error: %s\n", conf.Name, n, err)
					} else {
						break
					}
				}
				if err != nil {
					unRegistryFailed = true
				}
			}(service)
		}
	}
	wg.Wait()
	if unRegistryFailed {
		logrus.Errorln("unregister server info from registration center failed")
	}
	s.server.GracefulStop()
}

//func initServer() (client *zk.Client) {
//	if serverConf.Conf.Zk.Servers != "" {
//		initSuccess := true
//		client, err := zk.New(serverConf.Conf.Zk.Servers)
//		if nil != err {
//			logrus.Fatalln("connect zk failed")
//
//		}
//		defer func() {
//			if !initSuccess {
//				client.Close()
//			}
//		}()
//	} else {
//		logrus.Infoln("no zk config")
//	}
//	return
//}
