// copy the code from grpc.balancer.go and modify it to suit weight roundrobin
package roundrobin

import (
	"google.golang.org/grpc/naming"
	"sync"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc"
	"errors"
	"context"
	"openWebSF/config"
	"openWebSF/balancer"
	"github.com/sirupsen/logrus"
)

func RoundRobin(r naming.Resolver, flag bool) grpc.Balancer {
	return &roundRobin{
		r:      r,
		weight: flag,
	}
}

type roundRobin struct {
	r      naming.Resolver
	w      naming.Watcher
	addrs  []*balancer.AddrInfo // all the addresses the client should potentially connect
	mu     sync.Mutex
	addrCh chan []grpc.Address // the channel to notify gRPC internals the list of addresses the client should connect to.
	next   int                 // index of the next address to return for Get()
	waitCh chan struct{}       // the channel to block when there is no connected address available
	done   bool                // The Balancer is closed.
	weight bool
}

func (rr *roundRobin) watchAddrUpdates() error {
	method := "watchAddrUpdates"
	updates, err := rr.w.Next()
	if err != nil {
		grpclog.Warningf("grpc: the naming watcher stops working due to %v.", err)
		return err
	}
	rr.mu.Lock()
	defer rr.mu.Unlock()
	for _, update := range updates {
		addr := grpc.Address{
			Addr:     update.Addr,
			Metadata: update.Metadata,
		}
		switch update.Op {
		case naming.Add:
			var exist bool
			for _, v := range rr.addrs {
				if addr == v.Addr {
					exist = true
					grpclog.Infoln("grpc: The name resolver wanted to add an existing address: ", addr)
					break
				}
			}
			if exist {
				continue
			}
			weight := balancer.GetWeightByMetadata(addr.Metadata)
			logrus.Debugf("%s add %s weight[%d]", method, addr.Addr, weight)
			rr.addrs = append(rr.addrs, &balancer.AddrInfo{
				Addr: addr,
				Weight: weight,
				EffectiveWeight: weight,
			})
		case naming.Delete:
			for i, v := range rr.addrs {
				if addr == v.Addr {
					copy(rr.addrs[i:], rr.addrs[i+1:])
					rr.addrs = rr.addrs[:len(rr.addrs)-1]
					break
				}
			}
			rr.next = -1 // 后端服务地址减少时，重置 next
		case config.Modify:
			if !rr.weight {
				continue
			}
			for _, v := range rr.addrs {
				if addr.Addr == v.Addr.Addr {
					weight := balancer.GetWeightByMetadata(addr.Metadata)
					logrus.Debugf("%s modify %s weight[%d]", method, addr.Addr, weight)
					v.Weight = weight
					v.EffectiveWeight = weight
					break
				}
			}
		default:
			grpclog.Errorln("Unknown update.Op ", update.Op)
		}
	}
	// Make a copy of rr.addrs and write it onto rr.addrCh so that gRPC internals gets notified.
	open := make([]grpc.Address, len(rr.addrs))
	for i, v := range rr.addrs {
		open[i] = v.Addr
	}
	if rr.done {
		return grpc.ErrClientConnClosing
	}
	select {
	case <-rr.addrCh:
	default:
	}
	rr.addrCh <- open
	return nil
}

func (rr *roundRobin) Start(target string, config grpc.BalancerConfig) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if rr.done {
		return grpc.ErrClientConnClosing
	}
	if rr.r == nil {
		// resolver can't be nil, the roundrobin_v1 is just use when the resolver is exist
		return errors.New("resolver can't be nil")
	}
	w, err := rr.r.Resolve(target)
	if err != nil {
		return err
	}
	rr.w = w
	rr.addrCh = make(chan []grpc.Address)
	go func() {
		for {
			if err := rr.watchAddrUpdates(); err != nil {
				return
			}
		}
	}()
	return nil
}

// Up sets the connected state of addr and sends notification if there are pending
// Get() calls.
func (rr *roundRobin) Up(addr grpc.Address) func(error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	var cnt int
	for _, a := range rr.addrs {
		if a.Addr == addr {
			if a.Connected {
				return nil
			}
			a.Connected = true
		}
		if a.Connected {
			cnt++
		}
	}
	// addr is only one which is connected. Notify the Get() callers who are blocking.
	if cnt == 1 && rr.waitCh != nil {
		close(rr.waitCh)
		rr.waitCh = nil
	}
	return func(err error) {
		rr.down(addr, err)
	}
}

// down unsets the connected state of addr.
func (rr *roundRobin) down(addr grpc.Address, err error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	for _, a := range rr.addrs {
		if addr == a.Addr {
			a.Connected = false
			break
		}
	}
}

// Get returns the next addr in the rotation.
// modify this function to support weight roundrobin
func (rr *roundRobin) Get(ctx context.Context, opts grpc.BalancerGetOptions) (addr grpc.Address, put func(), err error) {
	var ch chan struct{}
	rr.mu.Lock()
	if rr.done {
		rr.mu.Unlock()
		err = grpc.ErrClientConnClosing
		return
	}

	addrs := balancer.GetAvailableAddrs(rr.addrs, rr.weight, true)
	if len(addrs) > 0 {
		addr = rr.selectOneAddr(addrs)
		rr.mu.Unlock()
		return
	}
	if !opts.BlockingWait {
		addrs = balancer.GetAvailableAddrs(rr.addrs, rr.weight, false)
		if len(addrs) == 0 {
			rr.mu.Unlock()
			err = status.Errorf(codes.Unavailable, "there is no address available")
			return
		}
		// Returns the next addr on rr.addrs for failfast RPCs.
		addr = rr.selectOneAddr(addrs)
		rr.mu.Unlock()
		return
	}
	// Wait on rr.waitCh for non-failfast RPCs.
	if rr.waitCh == nil {
		ch = make(chan struct{})
		rr.waitCh = ch
	} else {
		ch = rr.waitCh
	}
	rr.mu.Unlock()
	for {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			return
		case <-ch:
			rr.mu.Lock()
			if rr.done {
				rr.mu.Unlock()
				err = grpc.ErrClientConnClosing
				return
			}

			addrs := balancer.GetAvailableAddrs(rr.addrs, rr.weight, true)
			if len(addrs) > 0 {
				addr = rr.selectOneAddr(addrs)
				rr.mu.Unlock()
				return
			}

			// The newly added addr got removed by Down() again.
			if rr.waitCh == nil {
				ch = make(chan struct{})
				rr.waitCh = ch
			} else {
				ch = rr.waitCh
			}
			rr.mu.Unlock()
		}
	}
}

func (rr *roundRobin) Notify() <-chan []grpc.Address {
	return rr.addrCh
}

func (rr *roundRobin) Close() error {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if rr.done {
		return errors.New("grpc: balancer is closed")
	}
	rr.done = true
	if rr.w != nil {
		rr.w.Close()
	}
	if rr.waitCh != nil {
		close(rr.waitCh)
		rr.waitCh = nil
	}
	if rr.addrCh != nil {
		close(rr.addrCh)
	}
	return nil
}

// len(addrs) must bigger than 0
func (rr *roundRobin) selectOneAddr(addrs []*balancer.AddrInfo) grpc.Address {
	if len(addrs) == 1 {
		return addrs[0].Addr
	}

	// 基于权重
	if rr.weight {
		total := 0
		var selected *balancer.AddrInfo
		for _, addr := range addrs {
			addr.CurrentWeight += addr.EffectiveWeight
			total += addr.EffectiveWeight
			if selected == nil || selected.CurrentWeight < addr.CurrentWeight {
				selected = addr
			}
		}
		selected.CurrentWeight -= total
		return selected.Addr
	}
	// 不基于权重
	rr.next++
	if rr.next >= len(addrs) {
		rr.next = 0
	}
	return addrs[rr.next].Addr
}