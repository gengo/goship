package goship

import (
	"fmt"
	"path/filepath"

	"github.com/coreos/go-etcd/etcd"
)

// gitHubPaginationLimit is the default pagination limit for requests to the GitHub API that return multiple items.
const (
	gitHubPaginationLimit = 30
	pivotalCommentURL     = "https://www.pivotaltracker.com/services/v5/projects/%s/stories/%s/comments"
	gitHubAPITokenEnvVar  = "GITHUB_API_TOKEN"
)

// Host stores information on a host, such as URI and the latest commit revision.
type Host struct {
	URI             string
	LatestCommit    string
	GitHubCommitURL string
	GitHubDiffURL   string
	ShortCommitHash string
}

// Environment stores information about an individual environment, such as its name and whether it is deployable.
type Environment struct {
	Name               string
	Deploy             string
	RepoPath           string
	Hosts              []Host
	Branch             string
	Revision           string
	LatestGitHubCommit string
	IsDeployable       bool
}

// Project stores information about a GitHub project, such as its GitHub URL and repo name.
type Project struct {
	Name         string
	GitHubURL    string
	RepoName     string
	RepoOwner    string
	Environments []Environment
}

// Sort interface for sorting projects
//type ByName []Project

//func (slice ByName) Len() int           { return len(slice) }
//func (slice ByName) Less(i, j int) bool { return slice[i].Name < slice[j].Name }
//func (slice ByName) Swap(i, j int)      { slice[i], slice[j] = slice[j], slice[i] }

// gitHubCommitURL takes a project and returns the GitHub URL for its latest commit hash.
func (h *Host) LatestGitHubCommitURL(p Project) string {
	return fmt.Sprintf("%s/commit/%s", p.GitHubURL, h.LatestCommit)
}

// gitHubDiffURL takes a project and an environment and returns the GitHub diff URL
// for the latest commit on the host compared to the latest commit on GitHub.
func (h *Host) LatestGitHubDiffURL(p Project, e Environment) string {
	var s string
	if h.LatestCommit != e.LatestGitHubCommit {
		s = fmt.Sprintf("%s/compare/%s...%s", p.GitHubURL, h.LatestCommit, e.LatestGitHubCommit)
	}
	return s
}

// Deployable returns true if the latest commit for any of the hosts in an environment
// differs from the latest commit on GitHub, and false if all of the commits match.
func (e *Environment) Deployable() bool {
	for _, h := range e.Hosts {
		if e.LatestGitHubCommit != h.LatestCommit {
			return true
		}
	}
	return false
}

// ShortCommitHash returns a shortened version of the latest commit hash on a host.
func (h *Host) LatestShortCommitHash() string {
	if len(h.LatestCommit) == 0 {
		return ""
	}
	return h.LatestCommit[:7]
}

// config contains the information from config.yml.
type Config struct {
	Projects   []Project
	DeployUser string
	Notify     string
	Pivotal    *PivotalConfiguration
}

type PivotalConfiguration struct {
	Project string
	Token   string
}

type ETCDInterface interface {
	Get(string, bool, bool) (*etcd.Response, error)
}

// ProjectFromName takes a project name as a string and returns
// a Project by that name if it can find one.
func ProjectFromName(projects []Project, projectName string) (*Project, error) {
	for _, project := range projects {
		if project.Name == projectName {
			return &project, nil
		}
	}
	return nil, fmt.Errorf("No project found: %s", projectName)
}

// EnvironmentFromName takes an environment and project name as a string and returns
// an Environment by the given environment name under a project with the given
// project name if it can find one.
func EnvironmentFromName(projects []Project, projectName, environmentName string) (*Environment, error) {
	p, err := ProjectFromName(projects, projectName)
	if err != nil {
		return nil, err
	}
	for _, environment := range p.Environments {
		if environment.Name == environmentName {
			return &environment, nil
		}
	}
	return nil, fmt.Errorf("No environment found: %s", environmentName)
}

// connects to ETCD and returns the appropriate structs and strings.
func ParseETCD(client ETCDInterface) (c Config, err error) {
	baseInfo, err := client.Get("/", false, false)
	if err != nil {
		return c, err
	}
	deployUser := ""
	pivotalProject := ""
	token := ""
	notify := ""
	for _, b := range baseInfo.Node.Nodes {
		switch filepath.Base(b.Key) {
		case "deploy_user":
			deployUser = b.Value
		case "pivotal_project":
			pivotalProject = filepath.Base(b.Value)
		case "token":
			token = filepath.Base(b.Value)
		case "notify":
			notify = filepath.Base(b.Value)
		}
	}

	allProjects := []Project{}
	//  Get Projects //
	projectNodes, err := client.Get("/projects", false, false)
	if err != nil {
		return c, err
	}
	for _, p := range projectNodes.Node.Nodes {
		name := filepath.Base(p.Key)
		projectNode, err := client.Get("/projects/"+name, false, false)
		if err != nil {
			return c, err
		}
		projectInfo := projectNode.Node.Nodes
		repoOwner := ""
		repoName := ""
		for _, k := range projectInfo {
			switch filepath.Base(k.Key) {
			case "repo_owner":
				repoOwner = filepath.Base(k.Value)
			case "repo_name":
				repoName = filepath.Base(k.Value)
			}
		}
		githubURL := fmt.Sprintf("https://github.com/%s/%s", repoOwner, repoName)
		proj := Project{Name: name, GitHubURL: githubURL, RepoName: repoName, RepoOwner: repoOwner}
		environments, err := client.Get("/projects/"+name+"/environments", false, false)
		if err != nil {
			return c, err
		}
		allEnvironments := []Environment{}
		for _, e := range environments.Node.Nodes {
			envSettings, err := client.Get("/projects/"+name+"/environments/"+filepath.Base(e.Key), false, false)
			if err != nil {
				return c, err
			}
			envName := filepath.Base(e.Key)
			revision := "head"
			branch := "master"
			deploy := ""
			repoPath := ""
			for _, n := range envSettings.Node.Nodes {
				switch filepath.Base(n.Key) {
				case "revision":
					revision = n.Value
				case "branch":
					branch = n.Value
				case "deploy":
					deploy = n.Value
				case "repo_path":
					repoPath = n.Value
				}
			}
			//  Get Hosts per Environment.
			hosts, err := client.Get("/projects/"+name+"/environments/"+envName+"/hosts", false, false)
			if err != nil {
				return c, err
			}
			allHosts := []Host{}
			for _, h := range hosts.Node.Nodes {
				host := Host{URI: filepath.Base(h.Key)}
				allHosts = append(allHosts, host)
			}
			env := Environment{Name: envName, Deploy: deploy, RepoPath: repoPath, Branch: branch, Revision: revision}
			env.Hosts = allHosts
			allEnvironments = append(allEnvironments, env)
		}
		if err != nil {
			fmt.Printf("Skipping Project: %s", err)
			continue
		}
		proj.Environments = allEnvironments
		allProjects = append(allProjects, proj)
	}
	piv := new(PivotalConfiguration)
	piv.Project = pivotalProject
	piv.Token = token
	c.Projects = allProjects
	c.DeployUser = deployUser
	c.Notify = notify
	c.Pivotal = piv

	return c, err
}
