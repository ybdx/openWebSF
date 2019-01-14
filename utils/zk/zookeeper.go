package zk

import (
	"errors"
	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/sirupsen/logrus"
	"openWebSF/store/zookeeper"
	"strings"
	"sync"
)

var registerZkOnce = sync.Once{}

type Client struct {
	store   store.Store
	addrs   []string
	keyMap  map[string]*ZkKeyValue
	kvNames []string
}

func New(servers string) (*Client, error) {
	return initClient(servers)
}

func initClient(servers string) (*Client, error) {
	registerZkOnce.Do(func() {
		zookeeper.Register()
	})

	if "" == servers {
		err := errors.New("parameter can't be empty")
		logrus.Errorln(err)
		return nil, err
	}

	serverList := strings.Split(servers, ",")

	storeZk, err := libkv.NewStore(zookeeper.ZK_NEW, serverList, nil)
	if nil != err {
		logrus.Errorf("connected zookeeper %s failed", serverList, err)
		return nil, err
	}
	return &Client{
		store:   storeZk,
		addrs:   serverList,
		keyMap:  make(map[string]*ZkKeyValue),
		kvNames: make([]string, 0, 3),
	}, nil
}

func GetStore(servers string) (store.Store, error) {
	if client, err := initClient(servers); err != nil {
		return nil, err
	} else {
		return client.store, nil
	}
}

func (c *Client) Get(key string) (*store.KVPair, error) {
	return c.store.Get(key)
}

func (c *Client) Put(key string, value string) error {
	return c.store.Put(key, []byte(value), nil)
}

func (c *Client) Delete(key string) error {
	return c.store.Delete(key)
}

func (c *Client) List(key string) ([]*store.KVPair, error) {
	return c.store.List(key)
}

func (c *Client) Watch(key string, stopCh <-chan struct{}) (<-chan *store.KVPair, error) {
	return c.store.Watch(key, stopCh)
}

func (c *Client) WatchTree(directory string, stopCh <-chan struct{}) (<-chan []*store.KVPair, error) {
	return c.store.WatchTree(directory, stopCh)
}

func (c *Client) Close() {
	logrus.Infof("closing zookeeper %v connections", c.addrs)
	c.store.Close()
}
