package resolver

import (
	"github.com/docker/libkv/store"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/resolver"
	"net/url"
	"openWebSF/utils"
	"openWebSF/utils/zk"
	"time"
)

const scheme = "zookeeper"

type zookeeperBuilder struct {
	name string
}

func (zkb *zookeeperBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOption) (resolver.Resolver, error) {
	r := &zookeeperResolver{
		target:      target,
		cc:          cc,
		serviceName: zkb.name,
	}
	if nil == r.zk {
		if cli, err := zk.New(target.Endpoint); err != nil {
			logrus.Fatalln("libkv NewStore failed:", err)
			return nil, err
		} else {
			r.zk = cli
		}
	}

	go r.watch()
	return r, nil
}

func (zkb *zookeeperBuilder) Scheme() string {
	return scheme
}

type zookeeperResolver struct {
	target      resolver.Target
	cc          resolver.ClientConn
	serviceName string
	zk          *zk.Client
}

func (*zookeeperResolver) ResolveNow(o resolver.ResolveNowOption) {}

func (*zookeeperResolver) Close() {}

func newBuilder(serviceName string) *zookeeperBuilder {
	return &zookeeperBuilder{
		name: serviceName,
	}
}

func (r *zookeeperResolver) watch() {
	prefix := utils.ServicePrefix(r.serviceName)
	if pairs, err := r.zk.List(prefix); err == store.ErrKeyNotFound {
		logrus.Errorf("watcher list %s failed, error: %v", prefix, err)
		time.Sleep(3 * time.Second)
		return
	} else if err != nil {
		logrus.Errorf("watcher list %s failed, error: %v", prefix, err)
		return
	} else {
		addrs := make([]resolver.Address, 0)
		for _, v := range pairs {
			addr, metadata, err := getServerInfo(v)
			if err != nil {
				continue
			}
			addrs = append(addrs, resolver.Address{
				Addr:     addr,
				Metadata: metadata,
				Type:     resolver.Backend,
			})
		}
		r.cc.NewAddress(addrs)
	}
	stopCh := make(chan struct{})
	if event, err := r.zk.WatchTree(prefix, stopCh); err != nil {
		logrus.Errorf("watcher watchtree key %s failed, error: %v\n", prefix, err)
		return
	} else {
		addrs := getUpdates(event)
		r.cc.NewAddress(addrs)
	}

}

func getUpdates(kv <-chan []*store.KVPair) []resolver.Address {
	select {
	case pairs := <-kv:
		updates := make([]resolver.Address, 0)
		for _, pair := range pairs {
			addr, metadata, err := getServerInfo(pair)
			if err != nil {
				continue
			}
			updates = append(updates, resolver.Address{
				Addr:     addr,
				Type:     resolver.Backend,
				Metadata: metadata,
			})
		}
		return updates
	}
}

func getServerInfo(pair *store.KVPair) (string, string, error) {
	key, err := url.QueryUnescape(pair.Key)
	if err != nil {
		logrus.Errorf("url.QueryUnescape failed.", err)
		return "", "", err
	}
	return key, string(pair.Value), nil
}

func Init(serviceName string) {
	resolver.Register(newBuilder(serviceName))
}
