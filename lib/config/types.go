package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/gengo/goship/lib/pivotal"
	"github.com/golang/glog"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

const (
	gitHubAPITokenEnvVar = "GITHUB_API_TOKEN"
)

// Config is a set of Goship configurations
type Config struct {
	Projects   []Project             `json:"-" yaml:"projects,omitempty"`
	DeployUser string                `json:"deploy_user" yaml:"deploy_user"`
	Notify     string                `json:"notify" yaml:"notify"`
	Pivotal    *PivotalConfiguration `json:"pivotal,omitempty" yaml:"pivotal,omitempty"`
}

// Project stores information about a GitHub project, such as its GitHub URL and repo name, and a list of extra columns (PluginColumns)
type Project struct {
	Name         string        `json:"-" yaml:"name"`
	RepoName     string        `json:"repo_name" yaml:"repo_name"`
	RepoOwner    string        `json:"repo_owner" yaml:"repo_owner"`
	Environments []Environment `json:"-" yaml:"envs"`
	TravisToken  string        `json:"travis_token" yaml:"travis_token"`
}

// Environment stores information about an individual environment, such as its name and whether it is deployable.
type Environment struct {
	Name     string   `json:"-" yaml:"name"`
	Deploy   string   `json:"deploy" yaml:"deploy"`
	RepoPath string   `json:"repo_path" yaml:"repo_path"`
	Hosts    []string `json:"hosts" yaml:"hosts,omitempty"`
	Branch   string   `json:"branch" yaml:"branch"`
	Comment  string   `json:"comment" yaml:"comment"`
	IsLocked bool     `json:"is_locked,omitempty" yaml:"is_locked,omitempty"`
}

// PivotalConfiguration used to store Pivotal interface
type PivotalConfiguration struct {
	Token string `json:"token" yaml:"token"`
}

func PostToPivotal(piv *PivotalConfiguration, env, owner, name, latest, current string) error {
	layout := "2006-01-02 15:04:05"
	timestamp := time.Now()
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		layout += " (UTC)"
		glog.Error("time zone information for Asia/Tokyo not found")
	} else {
		layout += " (JST)"
		timestamp = timestamp.In(loc)
	}
	ids, err := GetPivotalIDFromCommits(owner, name, latest, current)
	if err != nil {
		return err
	}
	pivClient := pivotal.NewClient(piv.Token)
	for _, id := range ids {
		project, err := pivClient.FindProjectForStory(id)
		if err != nil {
			glog.Errorf("error getting project for story %d: %v", id, err)
			continue
		}
		m := fmt.Sprintf("Deployed to %s: %s", env, timestamp.Format(layout))
		go pivClient.AddComment(id, project, m)
		if env == "live" {
			year, week := time.Now().ISOWeek()
			label := fmt.Sprintf("released_w%d/%d", week, year)
			go pivClient.AddLabel(id, project, label)
		}
	}
	return nil
}

func appendIfUnique(list []int, elem int) []int {
	for _, item := range list {
		if item == elem {
			return list
		}
	}
	return append(list, elem)
}

func GetPivotalIDFromCommits(owner, repoName, latest, current string) ([]int, error) {
	// gets a list pivotal IDs from commit messages from repository based on latest and current commit
	gt := os.Getenv(gitHubAPITokenEnvVar)
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: gt})
	c := github.NewClient(oauth2.NewClient(oauth2.NoContext, ts))
	comp, _, err := c.Repositories.CompareCommits(owner, repoName, current, latest)
	if err != nil {
		return nil, err
	}
	pivRE, err := regexp.Compile("\\[.*#(\\d+)\\].*")
	if err != nil {
		return nil, err
	}
	var pivotalIDs []int
	for _, commit := range comp.Commits {
		cmi := *commit.Commit
		cm := *cmi.Message
		ids := pivRE.FindStringSubmatch(cm)
		if ids != nil {
			id := ids[1]
			n, err := strconv.Atoi(id)
			if err != nil {
				return nil, err
			}
			pivotalIDs = appendIfUnique(pivotalIDs, n)
		}
	}
	return pivotalIDs, nil
}

// ProjectFromName takes a project name as a string and returns
// a project by that name if it can find one.
func ProjectFromName(projects []Project, projectName string) (Project, error) {
	for _, project := range projects {
		if project.Name == projectName {
			return project, nil
		}
	}
	return Project{}, fmt.Errorf("No project found: %s", projectName)
}

// EnvironmentFromName takes an environment and project name as a string and returns
// an environment by the given environment name under a project with the given
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

// ETCDInterface emulates ETCD to allow testing
type ETCDInterface interface {
	Get(string, bool, bool) (*etcd.Response, error)
	Set(string, string, uint64) (*etcd.Response, error)
}
