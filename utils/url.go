package utils

import (
	"openWebSF/config"
	"fmt"
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
