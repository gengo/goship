package config

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
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
	Comment            string
	pivotalCommentURL  string
	IsLocked           bool
}

// Column is an interface that demands a RenderHeader and RenderDetails method to be able to generate a table column (with header and body)
// See templates/index.html to see how the Header and Render methods are used
type Column interface {
	// RenderHeader() returns a HTML template that should render a <th> element
	RenderHeader() (template.HTML, error)
	// RenderDetail() returns a HTML template that should render a <td> element
	RenderDetail() (template.HTML, error)
}

// Project stores information about a GitHub project, such as its GitHub URL and repo name, and a list of extra columns (PluginColumns)
type Project struct {
	PluginColumns []Column
	Name          string
	GitHubURL     string
	RepoName      string
	RepoOwner     string
	Environments  []Environment
}

func (p *Project) AddPluginColumn(c Column) {
	p.PluginColumns = append(p.PluginColumns, c)
}

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
	for _, id := range ids {
		m := fmt.Sprintf("Deployed to %s: %s", env, timestamp.Format(layout))
		go PostPivotalComment(id, m, piv)
	}
	return nil
}

func appendIfUnique(list []string, elem string) []string {
	for _, item := range list {
		if item == elem {
			return list
		}
	}
	return append(list, elem)
}

func GetPivotalIDFromCommits(owner, repoName, latest, current string) ([]string, error) {
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
	var pivotalIDs []string
	for _, commit := range comp.Commits {
		cmi := *commit.Commit
		cm := *cmi.Message
		ids := pivRE.FindStringSubmatch(cm)
		if ids != nil {
			id := ids[1]
			pivotalIDs = appendIfUnique(pivotalIDs, id)
		}
	}
	return pivotalIDs, nil
}

func PostPivotalComment(id string, m string, piv *PivotalConfiguration) (err error) {
	p := url.Values{}
	p.Set("text", m)
	req, err := http.NewRequest("POST", fmt.Sprintf(pivotalCommentURL, piv.Project, id), nil)
	if err != nil {
		glog.Errorf("could not form put request to Pivotal: %v", err)
		return err
	}
	req.URL.RawQuery = p.Encode()
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-TrackerToken", piv.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		glog.Errorf("could not make put request to Pivotal: %v", err)
		return err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		glog.Errorf("non-200 Response from Pivotal API: %s %s ", resp.Status, body)
	}
	return nil
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
	ETCDClient ETCDInterface
}

// PivotalConfiguration used to store Pivotal interface
type PivotalConfiguration struct {
	Project string
	Token   string
}

// ETCDInterface emulates ETCD to allow testing
type ETCDInterface interface {
	Get(string, bool, bool) (*etcd.Response, error)
	Set(string, string, uint64) (*etcd.Response, error)
}

// ProjectFromName takes a project name as a string and returns
// a project by that name if it can find one.
func ProjectFromName(projects []Project, projectName string) (*Project, error) {
	for _, project := range projects {
		if project.Name == projectName {
			return &project, nil
		}
	}
	return nil, fmt.Errorf("No project found: %s", projectName)
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

// ParseETCD connects to ETCD and returns the appropriate structs and strings.
func ParseETCD(client ETCDInterface) (c Config, err error) {
	baseInfo, err := client.Get("/", false, false)
	if err != nil {
		return c, err
	}
	deployUser := ""
	pivotalProject := ""
	pivotalToken := ""
	notify := ""
	for _, b := range baseInfo.Node.Nodes {
		switch filepath.Base(b.Key) {
		case "deploy_user":
			deployUser = b.Value
		case "pivotal_project":
			pivotalProject = b.Value
		case "pivotal_token":
			pivotalToken = b.Value
		case "notify":
			notify = b.Value
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
			isLocked := false
			comment := ""
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
				case "locked":
					nv, err := strconv.ParseBool(n.Value)
					if err != nil {
						fmt.Printf("Error parsing isLocked %s - Setting to unlocked: Please make sure environment has <locked> field", err)
						isLocked = false
					}
					isLocked = nv
				case "comment":
					comment = n.Value
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
			env := Environment{Name: envName, Deploy: deploy, RepoPath: repoPath, Branch: branch, Revision: revision, IsLocked: isLocked, Comment: comment}
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
	piv.Token = pivotalToken
	c.Projects = allProjects
	c.DeployUser = deployUser
	c.Notify = notify
	c.Pivotal = piv
	c.ETCDClient = client
	return c, err
}
