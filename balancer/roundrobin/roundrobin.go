package roundrobin

import (
	"context"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/resolver"
	ub "openWebSF/balancer"
	"sync"
)

const Name = "roundrobin_new"

func newBuilder(flag bool) balancer.Builder {
	return base.NewBalancerBuilderWithConfig(Name, &rrPickerBuilder{weight: flag}, base.Config{HealthCheck: true})
}

func Init(flag bool) string {
	balancer.Register(newBuilder(flag))
	return Name
}

type rrPickerBuilder struct {
	r resolver.Resolver
	weight bool // 是否采用权重进行负载均衡
}

func (rr *rrPickerBuilder) Build(readySCs map[resolver.Address]balancer.SubConn) balancer.Picker {
	grpclog.Infof("roundrobinPicker: newPicker called with readySCs: %v", readySCs)
	var scs []balancer.SubConn
	for _, sc := range readySCs {
		scs = append(scs, sc)
	}
	return &rrPicker{
		subConns: scs,
		readySCs: readySCs,
		weight: rr.weight,
	}
}

type rrPicker struct {
	// subConns is the snapshot of the roundrobin balancer when this picker was
	// created. The slice is immutable. Each Get() will do a round robin
	// selection from it and return the selected SubConn.
	subConns []balancer.SubConn
	readySCs map[resolver.Address]balancer.SubConn

	weight bool
	mu   sync.Mutex
	next int
}

func (p *rrPicker) Pick(ctx context.Context, opts balancer.PickOptions) (balancer.SubConn, func(balancer.DoneInfo), error) {
	if len(p.subConns) <= 0 {
		return nil, nil, balancer.ErrNoSubConnAvailable
	}

	// 基于权重
	if p.weight {
		p.mu.Lock()
		addrInfo := ub.TransformReadySCs(p.readySCs)
		sc := p.selectOneAddr(addrInfo)
		p.mu.Unlock()
		return sc, nil, nil
	}

	// 不基于权重
	p.mu.Lock()
	sc := p.subConns[p.next]
	p.next = (p.next + 1) % len(p.subConns)
	p.mu.Unlock()
	return sc, nil, nil
}

func (p *rrPicker) selectOneAddr(addrInfo []*ub.AddrInfoNew) balancer.SubConn {
	if len(addrInfo) == 1 {
		return addrInfo[0].SubConn
	}

	var selected *ub.AddrInfoNew
	total := 0
	for _, info := range addrInfo {
		info.CurrentWeight += info.Weight
		total += info.EffectiveWeight
		if selected == nil || selected.CurrentWeight < info.CurrentWeight {
			selected = info
		}
	}
	selected.CurrentWeight -= total
	return selected.SubConn
}
