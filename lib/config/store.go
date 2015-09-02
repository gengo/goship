package config

import (
	"encoding/json"
	"path"

	"github.com/golang/glog"
)

// Store stores "cfg" to etcd
func Store(client ETCDInterface, cfg Config) error {
	buf, err := json.Marshal(cfg)
	if err != nil {
		glog.Errorf("Failed to marshal global config: %v", err)
		return err
	}
	if _, err := client.Set("/goship/config", string(buf), 0); err != nil {
		glog.Errorf("Failed to store global config: %v", err)
		return err
	}
	for _, p := range cfg.Projects {
		if err := storeProject(client, p, "/goship"); err != nil {
			return err
		}
	}
	return nil
}

func storeProject(client ETCDInterface, p Project, base string) error {
	buf, err := json.Marshal(p)
	if err != nil {
		glog.Errorf("Failed to marshal project config of %s: %v", p.Name, err)
		return err
	}
	dir := path.Join(base, "projects", p.Name)
	if _, err := client.Set(path.Join(dir, "config"), string(buf), 0); err != nil {
		glog.Errorf("Failed to store project config of %s: %v", p.Name, err)
		return err
	}
	for _, env := range p.Environments {
		if err := storeEnvironment(client, env, path.Join(dir, "environments")); err != nil {
			return err
		}
	}
	return nil
}

func storeEnvironment(client ETCDInterface, env Environment, dir string) error {
	buf, err := json.Marshal(env)
	if err != nil {
		glog.Errorf("Failed to marshal environment config of %s: %v", env.Name, err)
		return err
	}
	if _, err := client.Set(path.Join(dir, env.Name), string(buf), 0); err != nil {
		glog.Errorf("Failed to store environment config of %s: %v", env.Name, err)
		return err
	}
	return nil
}
