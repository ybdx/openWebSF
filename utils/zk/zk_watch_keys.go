package zk

import "sync"

type ZkKeyValue struct {
	sync.RWMutex
	Name    string // 自己定义的名称
	Key     string
	Value   interface{}
	resChan <-chan interface{}
}
