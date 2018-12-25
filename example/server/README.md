服务启动如下：
```
	helloService := server.ServiceConfig{
		Name:    *service1,
		RegisterService: pb.RegisterHelloServiceServer,
		Server:   &service.HelloServer{},
	}
	server.NewServer().
		Register(helloService).
		Start(*port)
```

go run server.go -c ./service/conf/dev.yaml
