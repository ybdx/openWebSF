package random

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/naming"
	"math/rand"
	"time"
	"sync"
	"errors"
	"log"
	"github.com/sirupsen/logrus"
	"openWebSF/balancer"
	"openWebSF/config"
	"context"
)

// Random create a random balancer, if weight is true the balancer will
// consider the server's weight
func Random(r naming.Resolver, weight bool) grpc.Balancer {
	return &random{
		r:      r,
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())),
		weight: weight,
	}
}

type random struct {
	sync.Mutex
	rand   *rand.Rand
	r      naming.Resolver
	w      naming.Watcher
	addrs  []*balancer.AddrInfo         // all the addresses the client should potentially connect
	addrCh chan []grpc.Address // the channel to notify gRPC internals the list of addresses the client should connect to.
	waitCh chan struct{}       // the channel to block when there is no connected address available
	done   bool                // The Balancer is closed.
	weight bool                // 是否按照权重做负载均衡
}

func (b *random) Start(target string, config grpc.BalancerConfig) error {
	b.Lock()
	if b.done {
		return grpc.ErrClientConnClosing
	}
	if b.r == nil {
		return errors.New("resolver can't be nil")
	}
	w, err := b.r.Resolve(target)
	if err != nil {
		return err
	}
	b.w = w
	b.addrCh = make(chan []grpc.Address)
	b.Unlock()
	go func() {
		for {
			if err := b.watchAddrUpdates(); err != nil {
				return
			}
		}
	}()
	return nil
}

func (b *random) Up(addr grpc.Address) (down func(error)) {
	b.Lock()
	defer b.Unlock()
	var cnt int
	for _, a := range b.addrs {
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
	if cnt == 1 && b.waitCh != nil {
		close(b.waitCh)
		b.waitCh = nil
	}
	return func(err error) {
		b.down(addr, err)
	}
}

func (b *random) down(addr grpc.Address, err error) {
	b.Lock()
	defer b.Unlock()
	for _, a := range b.addrs {
		if addr == a.Addr {
			a.Connected = false
			break
		}
	}
}
func (b *random) Get(ctx context.Context, opts grpc.BalancerGetOptions) (addr grpc.Address, put func(), err error) {
	b.Lock()

	// get all connected address
	addrs := balancer.GetAvailableAddrs(b.addrs, b.weight, true)
	if len(addrs) > 0 {
		addr = b.selectOneAddr(addrs)
		b.Unlock()
		return
	}
	if !opts.BlockingWait {
		addrs := balancer.GetAvailableAddrs(b.addrs, b.weight, false)
		if len(addrs) == 0 {
			b.Unlock()
			err = errors.New("there is no address available")
			return
		}
		// Returns a random addr on b.addrs for failfast RPCs.
		addr = b.selectOneAddr(addrs)
		b.Unlock()
		return
	}
	// Wait on b.waitCh for non-failfast RPCs.
	if b.waitCh == nil {
		b.waitCh = make(chan struct{})
	}
	b.Unlock()
	for {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			return
		case <-b.waitCh:
			b.Lock()
			if b.done {
				b.Unlock()
				err = grpc.ErrClientConnClosing
				return
			}

			if len(b.addrs) > 0 {
				addrs :=balancer.GetAvailableAddrs(b.addrs, b.weight, true)
				if len(addrs) > 0 {
					addr = b.selectOneAddr(addrs)
					b.Unlock()
					return
				}
			}
			// The newly added addr got removed by Down() again.
			if b.waitCh == nil {
				b.waitCh = make(chan struct{})
			}
			b.Unlock()
		}
	}
}
func (b *random) Notify() <-chan []grpc.Address {
	return b.addrCh
}
func (b *random) Close() error {
	b.Lock()
	defer b.Unlock()
	b.done = true
	if b.w != nil {
		b.w.Close()
	}
	if b.waitCh != nil {
		close(b.waitCh)
		b.waitCh = nil
	}
	if b.addrCh != nil {
		close(b.addrCh)
	}
	return nil
}
func (b *random) watchAddrUpdates() error {
	updates, err := b.w.Next()
	if err != nil {
		log.Fatalln("grpc: the naming watcher stops working due to %v.\n", err)
		return err
	}
	b.Lock()
	defer b.Unlock()
	for _, update := range updates {
		addr := grpc.Address{
			Addr:     update.Addr,
			Metadata: update.Metadata,
		}
		switch update.Op {
		case naming.Add:
			var exist bool
			for _, v := range b.addrs {
				if addr == v.Addr {
					exist = true
					break
				}
			}
			if exist {
				continue
			}
			b.addrs = append(b.addrs, &balancer.AddrInfo{
				Addr:   addr,
				Weight:  balancer.GetWeightByMetadata(addr.Metadata),
			})
		case config.Modify:
			// 现在只会修改weight值，所以不根据权重做负载均衡时直接continue
			if !b.weight {
				continue
			}
			for _, v := range b.addrs {
				if v.Addr.Addr == addr.Addr {
					v.Weight = balancer.GetWeightByMetadata(addr.Metadata)
					break
				}
			}
		case naming.Delete:
			for i, v := range b.addrs {
				if addr.Addr == v.Addr.Addr {
					copy(b.addrs[i:], b.addrs[i+1:])
					b.addrs = b.addrs[:len(b.addrs)-1]
					break
				}
			}
		default:
			logrus.Warnln("Unknown update.Op ", update.Op)
		}
	}
	// Make a copy of b.addrs and write it onto b.addrCh so that gRPC internals gets notified.
	open := make([]grpc.Address, len(b.addrs))
	for i, v := range b.addrs {
		open[i] = v.Addr
	}
	if b.done {
		return grpc.ErrClientConnClosing
	}
	b.addrCh <- open
	return nil
}

// len(addrs) must bigger than 0
func (b *random) selectOneAddr(addrs []*balancer.AddrInfo) grpc.Address {
	if len(addrs) == 1 {
		return addrs[0].Addr
	}
	if b.weight {
		total := 0
		for _, v := range addrs {
			total += v.Weight
		}
		n := b.rand.Intn(total)
		sum := 0
		for _, v := range addrs {
			sum += v.Weight
			if n < sum {
				return v.Addr
			}
		}
		return addrs[len(addrs)-1].Addr
	}
	return addrs[b.rand.Intn(len(addrs))].Addr
}

