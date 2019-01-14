package resolver

import (
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/naming"
	"openWebSF/utils/zk"
)

type zookeeper struct {
	name string // 服务名
}

func ZookeeperResolve(name string) *zookeeper {
	return &zookeeper{
		name: name,
	}
}

func (r *zookeeper) Resolve(target string) (naming.Watcher, error) {
	cli, err := zk.New(target)
	if err != nil {
		logrus.Fatalln("connected to zk failed!")
		return nil, err
	}

	return NewWatcher(cli, r), nil
}
