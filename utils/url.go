package utils

import (
	"fmt"
	"openWebSF/config"
)

func ServicePrefix(name string, groups ...string) string {
	group := ""
	if len(groups) == 1 {
		group = groups[0]
	} else {
		group = config.Default.Group
	}
	return fmt.Sprintf("%s/%s/%s/%s", config.Default.Schema, group, name, config.Server)
}

func ServiceKey(name string, port int, groups ...string) string {
	return fmt.Sprintf("%s/%s:%d", ServicePrefix(name, groups...), config.Default.LocalIPv4, port)
}

func ClientPrefix(name string, groups ...string) string {
	group := ""
	if len(groups) == 1 {
		group = groups[0]
	} else {
		group = config.Default.Group
	}
	return fmt.Sprintf("%s/%s/%s/%s", config.Default.Schema, group, name, config.Client)
}

func ClientKey(name string, groups ...string) string {
	return fmt.Sprintf("%s/%s", ClientPrefix(name, groups...), config.Default.LocalIPv4)
}
