package config

import (
	"fmt"
)

const (
	Server = "s"
	Client = "c"
)
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

func (m MetaDataInner) String() string {
	return fmt.Sprintf("weight=%d&active=%d&owner=%s&lang=%s&pid=%d&user=%s", m.Weight, m.Active, m.Owner, m.Lang, m.Pid, m.User)
}