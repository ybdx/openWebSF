package serverConf

import (
	"flag"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

var Conf ConfType

type ConfType struct {
	AppName      string `yaml:"appName"`
	LimitQPS     int    `yaml:"limitQPS"`
	RegisterAddr string `yaml:"registry_addr"`
	Port         int    `yaml:"port"`
	Zk           zkConfig
	Owner        string
}

type zkConfig struct {
	Servers string
}

func GetConfFromFile() (conf ConfType) {
	file := ""
	for i := range os.Args {
		if os.Args[i] == "-c" && len(os.Args) > i+1 {
			file = os.Args[i+1]
			break
		}
	}
	if "" == file {
		logrus.Infoln("not specify config file")
		return
	}
	return parseFile(file)
}

func parseFile(file string) (conf ConfType) {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		logrus.Fatalf("read config file %s failed, error: %v", file, err)
	} else {
		logrus.Infof("read config file %s success", file)
	}
	if err := yaml.Unmarshal(content, &conf); err != nil {
		logrus.Fatalf("parse config file %s failed, error: %v", file, err)
	}
	filterConfig(&conf)
	return
}

func filterConfig(conf *ConfType) {
	if conf.AppName == "" {
		logrus.Fatalln("config file appName is empty")
	}
}

func init() {
	flag.String("c", "", "config file path")
	Conf = GetConfFromFile()
}
