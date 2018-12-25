Client Config参数介绍如下：
- Service

    服务名，客户端可以通过该服务名发现注册中心服务的地址
- Registry

    注册中心地址，采用直接连接的时候该字段为空
- DirectAddr

    采用直接连接的方式访问服务，Service字段为空时需要设置该字段
- Balancer

    采用的负载均衡，不设置时采用默认的WRoundRobin进行负载，若采用expreimental接口中的负载，此值需要设置
- Experimental

    是否采用expreimental API进行resolver and balancer, 值为false不采用， 默认false, 若采用，此值必须设置为true

客户端连接方式如下：
```
client.NewClient(client.ClientConfig{
        Service:  "serviceName",
        Registry: "127.0.0.1:9301",
        Balancer: client.RoundRobinExperimental,
        Experimental: true,
 })
```