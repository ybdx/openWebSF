package registry

import (
	"sync"
	"github.com/docker/libkv/store"
	"github.com/sirupsen/logrus"
	"openWebSF/utils/zk"
	"strings"
	"openWebSF/config"
	"openWebSF/utils"
	"time"
	"fmt"
)

type Registry struct {
	sync.Mutex
	store store.Store
}

func Register(servers string) *Registry {
	logrus.Debugln("new store for registration center, address:", servers)
	target := ParseTarget(servers)
	kvStore, err := zk.GetStore(target)
	if err != nil {
		logrus.Errorf("initial store to registration center %s failed, error: %v", servers, err)
		return nil
	}
	return &Registry{
		store: kvStore,
	}
}

func (r *Registry) RegisterService(serviceName string, port int, metadata config.MetaDataInner) error {
	key := utils.ServiceKey(serviceName, port)
	value := []byte(metadata.String())
	return r.register(key, value)
}

func (r *Registry) UnRegisterService(serviceName string, port int) error {
	key := utils.ServiceKey(serviceName, port)
	return r.unregister(key)
}

func (r *Registry) RegisterClient(serviceName string, pid int) error {
	key := utils.ClientKey(serviceName)
	value := []byte(fmt.Sprintf("%d", pid))
	return r.register(key, value)

}

func (r *Registry) UnRegisterClient(serviceName string) error {
	key := utils.ClientKey(serviceName)
	return r.unregister(key)
}


func (r *Registry) register(key string, value []byte) error {
	r.Lock()
	defer r.Unlock()
	if err := r.store.Delete(key); err != nil && err != store.ErrKeyNotFound {
		return err
	} else if err == nil {
		logrus.Debugf("key[%s] already registered, delete first", key)
	}
	if err := r.store.Put(key, value, &store.WriteOptions{TTL: time.Second}); err != nil {
		return err
	}
	logrus.Infof("register [%s] to registration center success", key)
	return nil
}

func (r *Registry) unregister(key string) error {
	r.Lock()
	defer r.Unlock()
	if err := r.store.Delete(key); err != nil {
		if err == store.ErrKeyNotFound {
			logrus.Warnf("unregister key[%s] failed, not found the key, this should not happen", key)
		}
		return err
	}
	logrus.Infof("unregister key[%s] succeed from to registry center")
	return nil
}

func split2(s, sep string) (string, string, bool) {
	spl := strings.SplitN(s, sep, 2)
	if len(spl) < 2 {
		return "", "", false
	}
	return spl[0], spl[1], true
}

func ParseTarget(target string) string {
	var ok bool
	_, endpoint, ok := split2(target, "://")
	if !ok {
		return target
	}
	_, endpoint, ok = split2(endpoint, "/")
	if !ok {
		return target
	}
	return endpoint
}

func (r *Registry) Close() {
	r.store.Close()
}