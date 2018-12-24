package config

import (
	"os"
	"net"
	"github.com/sirupsen/logrus"
)

const nodeClusterKey = "NODE_CLUSTER"
const DefaultGroup = "default"

type config struct {
	LocalIPv4      string
	LocalIPv4Bytes []byte
	Schema         string
	Namespace      string
	Group          string
	Lb             string
}

// 路径在zk中
var Default = config{
	Schema:    "osf",
	Namespace: "osf",
	Group:     DefaultGroup,
	Lb:        "",
}

func init() {
	getLocalIP()
	if env := os.Getenv(nodeClusterKey); env != "" {
		Default.Group = env
	}
}

// private IPv4
// Class        Starting IPAddress    Ending IP Address    # of Hosts
// A            10.0.0.0              10.255.255.255       16,777,216
// B            172.16.0.0            172.31.255.255       1,048,576
// C            192.168.0.0           192.168.255.255      65,536
// Link-local-u 169.254.0.0           169.254.255.255      65,536
// Link-local-m 224.0.0.0             224.0.0.255          256
// Local        127.0.0.0             127.255.255.255      16777216
func isIPUseful(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
		return false
	}
	ipv4 := ip.To4()
	if ipv4 == nil {
		return false
	}
	return true
}

func getLocalIP() {
	if addrs, err := net.InterfaceAddrs(); err != nil {
		logrus.Panicf("Get Local IP Failed, error is: % v\n", err)
	} else {
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				if isIPUseful(ipNet.IP) {
					Default.LocalIPv4 = ipNet.IP.String()
					Default.LocalIPv4Bytes = ipNet.IP.To4()
				}
			}
		}
	}
}
