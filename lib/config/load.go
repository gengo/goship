package config

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"
)

// Load loads a deployment configuration from etcd
func Load(client ETCDInterface) (Config, error) {
	resp, err := client.Get("/goship/config", false, false)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal([]byte(resp.Node.Value), &cfg); err != nil {
		glog.Errorf("Failed to unmarshal %s: %v", resp.Node.Value, err)
		return Config{}, err
	}
	if err := loadProjects(client, &cfg, "/goship"); err != nil {
		return Config{}, err
	}
	glog.V(2).Infof("Loaded config: %#v", cfg)
	return cfg, nil
}

func loadProjects(client ETCDInterface, cfg *Config, basePath string) error {
	projs, err := client.Get(path.Join(basePath, "projects"), false, true)
	if err != nil {
		return err
	}
	if !projs.Node.Dir {
		return fmt.Errorf("node %s must be a directory", projs.Node.Key)
	}
	for _, node := range projs.Node.Nodes {
		proj, err := loadProject(node)
		if err != nil {
			glog.Errorf("Skipping Project %s: %v", path.Base(node.Key), err)
			continue
		}
		cfg.Projects = append(cfg.Projects, proj)
	}
	return nil
}

func loadProject(node *etcd.Node) (Project, error) {
	name := path.Base(node.Key)
	var proj Project
	var envs *etcd.Node
	for _, child := range node.Nodes {
		switch path.Base(child.Key) {
		case "config":
			if err := json.Unmarshal([]byte(child.Value), &proj); err != nil {
				glog.Errorf("Failed to unmarshal %s: %v", child.Value, err)
				return Project{}, err
			}
		case "environments":
			envs = child
		}
	}
	if proj.RepoType == "" {
		proj.RepoType = RepoTypeGithub
	}
	if !proj.RepoType.Valid() {
		return Project{}, fmt.Errorf("invalid repo_type %q", proj.RepoType)
	}
	if proj.RepoType == RepoTypeDocker && proj.Source == nil {
		return Project{}, fmt.Errorf("source repo not configured in %s", name)
	}

	proj.Name = name
	if err := loadEnvironments(envs, &proj); err != nil {
		return Project{}, err
	}
	return proj, nil
}

func loadEnvironments(node *etcd.Node, proj *Project) error {
	if !node.Dir {
		return fmt.Errorf("node %s must be a directory", node.Key)
	}
	for _, child := range node.Nodes {
		env, err := loadEnvironment(child)
		if err != nil {
			return err
		}
		proj.Environments = append(proj.Environments, env)
	}
	return nil
}

func loadEnvironment(node *etcd.Node) (Environment, error) {
	var env Environment
	if err := json.Unmarshal([]byte(node.Value), &env); err != nil {
		glog.Errorf("Failed to unmarshal %s: %v", node.Value, err)
		return Environment{}, err
	}
	env.Name = path.Base(node.Key)
	if env.Branch == "" {
		env.Branch = "master"
	}
	return env, nil
}
