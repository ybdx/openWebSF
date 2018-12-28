package resolver

import (
	"openWebSF/utils/zk"
	"google.golang.org/grpc/naming"
	"openWebSF/utils"
	"github.com/docker/libkv/store"
	"github.com/sirupsen/logrus"
	"time"
	"openWebSF/config"
)

type watcher struct {
	zkClient   *zk.Client
	zkResolver *zookeeper
	servers    map[string]string
	event      <-chan []*store.KVPair
}

func NewWatcher(zkClient *zk.Client, zkResolver *zookeeper) (*watcher) {
	return &watcher{
		zkClient:   zkClient,
		zkResolver: zkResolver,
		servers:    make(map[string]string),
	}
}

func (w *watcher) Next() ([]*naming.Update, error) {
	if w.event == nil {
		prefix := utils.ServicePrefix(w.zkResolver.name)
		pairs, err := w.zkClient.List(prefix)
		if err == store.ErrKeyNotFound {
			logrus.Errorf("watcher list %s failed, error: %v====\n", prefix, err)
			time.Sleep(3 * time.Second)
			return nil, nil
		} else if nil != err {
			logrus.Errorf("watcher list %s failed, error: %v\n", prefix, err)
		}
		updates := make([]*naming.Update, len(pairs))
		for i, pair := range pairs {
			addr, mete, err := getServerInfo(pair)
			if err != nil {
				continue
			}
			updates[i] = &naming.Update{
				Op:       naming.Add,
				Addr:     addr,
				Metadata: mete,
			}
			w.servers[addr] = mete
		}
		stopCh := make(chan struct{})
		w.event, err = w.zkClient.WatchTree(prefix, stopCh)
		if err != nil {
			logrus.Errorf("watcher watchtree key %s failed, error: %v\n", prefix, err)
			return nil, err
		}
		return updates, nil
	}

	return w.getUpdate(w.event), nil
}

func (w *watcher) Close() {}

func (w *watcher) getUpdate(event <-chan []*store.KVPair) []*naming.Update {
	select {
	case pairs := <-event:
		updates := make([]*naming.Update, 0)
		currentServers := make(map[string]string)
		for _, pair := range pairs {
			addr, mete, err := getServerInfo(pair)
			if err != nil {
				continue
			}
			currentServers[addr] = mete
			v, ok := w.servers[addr]
			update := &naming.Update{
				Addr:     addr,
				Metadata: mete,
			}
			if ok && v != mete {
				update.Op = config.Modify
			} else if !ok {
				update.Op = naming.Add
			}
			updates = append(updates, update)
			w.servers[addr] = mete
		}

		for addr := range w.servers {
			if _, ok := currentServers[addr]; !ok {
				update := &naming.Update{
					Op:       naming.Delete,
					Addr:     addr,
					Metadata: w.servers[addr],
				}
				updates = append(updates, update)
				delete(w.servers, addr)
			}
		}
		return updates
	}
}
