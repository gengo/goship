package main

import (
	"fmt"
	"path"
	"strconv"

	"github.com/coreos/go-etcd/etcd"
	"github.com/gengo/goship/lib/config"
	"github.com/golang/glog"
)

// loadV1 loads a deployment configuration from etcd
func loadV1(client config.ETCDInterface) (config.Config, error) {
	baseInfo, err := client.Get("/", false, false)
	if err != nil {
		return config.Config{}, err
	}
	if !baseInfo.Node.Dir {
		return config.Config{}, fmt.Errorf("node %s must be a directory", baseInfo.Node.Key)
	}
	cfg := config.Config{
		Pivotal: new(config.PivotalConfiguration),
	}
	for _, b := range baseInfo.Node.Nodes {
		switch path.Base(b.Key) {
		case "deploy_user":
			cfg.DeployUser = b.Value
		case "pivotal_project":
			cfg.Pivotal.Project = b.Value
		case "pivotal_token":
			cfg.Pivotal.Token = b.Value
		case "notify":
			cfg.Notify = b.Value
		}
	}
	if err := loadProjects(client, &cfg); err != nil {
		return config.Config{}, err
	}
	return cfg, nil
}

func loadProjects(client config.ETCDInterface, cfg *config.Config) error {
	projs, err := client.Get("/projects", false, true)
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

func loadProject(node *etcd.Node) (config.Project, error) {
	proj := config.Project{Name: path.Base(node.Key)}
	for _, child := range node.Nodes {
		switch path.Base(child.Key) {
		case "repo_owner":
			proj.RepoOwner = path.Base(child.Value)
		case "repo_name":
			proj.RepoName = path.Base(child.Value)
		case "travis_token":
			proj.TravisToken = path.Base(child.Value)
		case "environments":
			if err := loadEnvironments(child, &proj); err != nil {
				return config.Project{}, err
			}
		}
	}
	return proj, nil
}

func loadEnvironments(node *etcd.Node, proj *config.Project) error {
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

func loadEnvironment(node *etcd.Node) (config.Environment, error) {
	env := config.Environment{
		Name:   path.Base(node.Key),
		Branch: "master",
	}
	for _, n := range node.Nodes {
		switch path.Base(n.Key) {
		case "branch":
			env.Branch = n.Value
		case "deploy":
			env.Deploy = n.Value
		case "repo_path":
			env.RepoPath = n.Value
		case "locked":
			locked, err := strconv.ParseBool(n.Value)
			if err != nil {
				glog.Errorf("Failed to parse 'locked' field %q. Assuming unlocked: %v", n.Value, err)
				continue
			}
			env.IsLocked = locked
		case "comment":
			env.Comment = n.Value
		case "hosts":
			if err := loadHosts(n, &env); err != nil {
				return config.Environment{}, err
			}
		}
	}
	return env, nil
}

func loadHosts(node *etcd.Node, env *config.Environment) error {
	if !node.Dir {
		return fmt.Errorf("node %s must be a directory", node.Key)
	}
	for _, h := range node.Nodes {
		env.Hosts = append(env.Hosts, path.Base(h.Key))
	}
	return nil
}
