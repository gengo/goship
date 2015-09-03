package main

import (
	"flag"
	"io/ioutil"
	"os"

	"github.com/coreos/go-etcd/etcd"
	"github.com/gengo/goship/lib/config"
	"github.com/golang/glog"
	yaml "gopkg.in/yaml.v2"
)

var (
	endpoint = flag.String("endpoinot", "http://localhost:4001", "etcd endpoint")
	dump     = flag.Bool("dump", false, "dumps configs from etcd")
	dumpV1   = flag.Bool("dump-v1", false, "same as -dump but reads from old structure of etcd directory")
	store    = flag.Bool("store", false, "store configs into etcd")
)

func dumpCfg(cfg config.Config, err error) error {
	if err != nil {
		glog.Errorf("Failed to load current configs: %v", err)
		return err
	}
	buf, err := yaml.Marshal(cfg)
	if err != nil {
		glog.Errorf("Failed to marshal config: %v", err)
		return err
	}
	_, err = os.Stdout.Write(buf)
	return err
}

func storeCfg(ecl *etcd.Client) error {
	buf, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		glog.Errorf("Failed to read config: %v", err)
		return err
	}
	var cfg config.Config
	if err := yaml.Unmarshal(buf, &cfg); err != nil {
		glog.Errorf("Failed to marshal config: %v", err)
		return err
	}
	return config.Store(ecl, cfg)
}

func main() {
	flag.Parse()
	defer glog.Flush()

	ecl := etcd.NewClient([]string{*endpoint})
	switch {
	case *dump:
		if err := dumpCfg(config.Load(ecl)); err != nil {
			glog.Fatal(err)
		}
	case *dumpV1:
		if err := dumpCfg(loadV1(ecl)); err != nil {
			glog.Fatal(err)
		}
	case *store:
		if err := storeCfg(ecl); err != nil {
			glog.Fatal(err)
		}
	default:
		glog.Errorf("either -dump, -dump-v1 or -store must be specified")
		flag.CommandLine.PrintDefaults()
		os.Exit(1)
	}
}
