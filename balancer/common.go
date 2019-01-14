package balancer

import (
	"github.com/sirupsen/logrus"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/resolver"
	"openWebSF/config"
)

type AddrInfo struct {
	Addr            grpc.Address
	Connected       bool
	Weight          int
	CurrentWeight   int
	EffectiveWeight int
}

type AddrInfoNew struct {
	Addr            string
	SubConn         balancer.SubConn
	Weight          int
	CurrentWeight   int
	EffectiveWeight int
}

func GetWeightByMetadata(meta interface{}) int {
	if metaStr, ok := meta.(string); ok {
		items := strings.Split(metaStr, "&")
		for _, item := range items {
			kv := strings.Split(item, "=")
			if len(kv) != 2 {
				logrus.Warnf("metadata[%s] format invalid\n", metaStr)
				continue
			}
			if kv[0] == "weight" {
				weight, err := strconv.Atoi(kv[1])
				if err != nil {
					logrus.Warnf("strconv.Atoi(%s) failed, use default value[%d] error: %s\n", kv[1], config.MetaDefaultWeight, err)
					return config.MetaDefaultWeight
				} else if weight < 0 {
					logrus.Warnf("get weight[%d] less than zero, use default value[%d]", weight, config.MetaDefaultWeight)
					return config.MetaDefaultWeight
				} else {
					return weight
				}
			}
		}
	} else {
		logrus.Warnf("metadata[%s] is not string\n", meta)
	}
	return config.MetaDefaultWeight
}

func GetAvailableAddrs(all []*AddrInfo, weight bool, connected bool) []*AddrInfo {
	addrs := make([]*AddrInfo, 0, len(all))
	for _, a := range all {
		if (!connected || a.Connected) && (!weight || a.Weight > 0) {
			addrs = append(addrs, a)
		}
	}
	return addrs
}

func TransformReadySCs(readySCs map[resolver.Address]balancer.SubConn) []*AddrInfoNew {
	addrInfo := make([]*AddrInfoNew, 0)
	for k, v := range readySCs {
		weight := GetWeightByMetadata(k.Metadata)
		info := &AddrInfoNew{
			Addr:            k.Addr,
			Weight:          weight,
			EffectiveWeight: weight,
			SubConn:         v,
		}
		addrInfo = append(addrInfo, info)
	}
	return addrInfo
}
