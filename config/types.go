package config

import (
	"fmt"
	"google.golang.org/grpc/naming"
)

const (
	Server = "s"
	Client = "c"
)

const EnvServerWeight = "SERVER_WEIGHT"    // 指定 server 权重

const (
	Modify naming.Operation = 0xFF //扩展naming.Operation
)

const MetaDefaultWeight = 100
const MetaLang = "go"
const MetaActiveOnline = 0
const MetaActiveOffline = 1

type MetaData struct {
	Owner string
}

type MetaDataInner struct {
	MetaData
	Weight int
	Active int
	Lang   string
	Pid    int
	User   string // 启动服务的用户名
}

var DefaultMetaDataInner = MetaDataInner{
	Weight: MetaDefaultWeight,
	Active: MetaActiveOnline,
	Lang:   MetaLang,
}

func (m MetaDataInner) String() string {
	return fmt.Sprintf("weight=%d&active=%d&owner=%s&lang=%s&pid=%d&user=%s", m.Weight, m.Active, m.Owner, m.Lang, m.Pid, m.User)
}