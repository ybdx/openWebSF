package roundrobin

import (
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/grpclog"
	"sync"
	"context"
)

const Name = "roundrobin_new"

func newBuilder(flag bool) balancer.Builder {
	return base.NewBalancerBuilderWithConfig(Name, &rrPickerBuilder{weight: flag}, base.Config{HealthCheck: true})
}

func Init(flag bool) {
	balancer.Register(newBuilder(flag))
}

type rrPickerBuilder struct {
	r resolver.Resolver
	weight bool
}

func (*rrPickerBuilder) Build(readySCs map[resolver.Address]balancer.SubConn) balancer.Picker {
	grpclog.Infof("roundrobinPicker: newPicker called with readySCs: %v", readySCs)
	var scs []balancer.SubConn
	for _, sc := range readySCs {
		scs = append(scs, sc)
	}
	return &rrPicker{
		subConns: scs,
		//readySCs: readySCs,
	}
}

type rrPicker struct {
	// subConns is the snapshot of the roundrobin balancer when this picker was
	// created. The slice is immutable. Each Get() will do a round robin
	// selection from it and return the selected SubConn.
	subConns []balancer.SubConn
	//readySCs map[resolver.Address]balancer.SubConn

	mu   sync.Mutex
	next int
}

func (p *rrPicker) Pick(ctx context.Context, opts balancer.PickOptions) (balancer.SubConn, func(balancer.DoneInfo), error) {
	if len(p.subConns) <= 0 {
		return nil, nil, balancer.ErrNoSubConnAvailable
	}

	p.mu.Lock()
	sc := p.subConns[p.next]
	p.next = (p.next + 1) % len(p.subConns)
	p.mu.Unlock()
	return sc, nil, nil
}
