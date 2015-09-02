package config

import (
	"fmt"
	"path"
	"strconv"

	"github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"
)

// SetComment will set the  comment field on an environment
func SetComment(client ETCDInterface, projectName, projectEnv, comment string) (err error) {
	projectString := fmt.Sprintf("/projects/%s/environments/%s/comment", projectName, projectEnv)
	// guard against empty values ( simple validation)
	if projectName == "" || projectEnv == "" {
		return fmt.Errorf("Missing parameters")
	}
	_, err = client.Set(projectString, comment, 0)
	return err
}

// LockEnvironment Locks or unlock an environment for deploy
func LockEnvironment(client ETCDInterface, projectName, projectEnv, lock string) (err error) {
	projectString := fmt.Sprintf("/projects/%s/environments/%s/locked", projectName, projectEnv)
	// guard against empty values ( simple validation)
	if projectName == "" || projectEnv == "" {
		return fmt.Errorf("Missing parameters")
	}
	_, err = client.Set(projectString, lock, 0)
	return err
}

// Load loads a deployment configuration from etcd
func Load(client ETCDInterface) (Config, error) {
	baseInfo, err := client.Get("/", false, false)
	if err != nil {
		return Config{}, err
	}
	if !baseInfo.Node.Dir {
		return Config{}, fmt.Errorf("node %s must be a directory", baseInfo.Node.Key)
	}
	cfg := Config{
		Pivotal: new(PivotalConfiguration),
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
		return Config{}, err
	}
	return cfg, nil
}

func loadProjects(client ETCDInterface, cfg *Config) error {
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

func loadProject(node *etcd.Node) (Project, error) {
	proj := Project{Name: path.Base(node.Key)}
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
				return Project{}, err
			}
		}
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
	env := Environment{
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
				return Environment{}, err
			}
		}
	}
	return env, nil
}

func loadHosts(node *etcd.Node, env *Environment) error {
	if !node.Dir {
		return fmt.Errorf("node %s must be a directory", node.Key)
	}
	for _, h := range node.Nodes {
		env.Hosts = append(env.Hosts, path.Base(h.Key))
	}
	return nil
}
