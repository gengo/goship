package main

// This is a simple quick script to take a goship config file and put into ETCD. Note: It does not wipe out your
// existing etcd setup.

import (
	"flag"
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"github.com/kylelemons/go-gypsy/yaml"
	"log"
	"strings"
)

var (
	ConfigFile = flag.String("c", "config.yml", "Path to data directory (default config.yml)")
	ETCDServer = flag.String("e", "http://127.0.0.1:4001", "Etcd Server (default http://127.0.0.1:4001")
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

type PivotalConfiguration struct {
	project string
	token   string
}

// config contains the information from config.yml.
type config struct {
	Projects   []Project
	DeployUser string
	Notify     string
	Pivotal    *PivotalConfiguration
}

// getYAMLString is a helper function for extracting strings from a yaml.Node.
func getYAMLString(n yaml.Node, key string) string {
	s, ok := n.(yaml.Map)[key].(yaml.Scalar)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s.String())
}

// parseYAMLEnvironment populates an Environment given a yaml.Node and returns the Environment.
func parseYAMLEnvironment(m yaml.Node, client *etcd.Client, proj_path string) Environment {
	e := Environment{}
	for k, v := range m.(yaml.Map) {
		e.Name = k
		proj_path = proj_path + "environments/" + e.Name + "/"
		log.Printf("Setting env name=> %s \n", proj_path)
		client.CreateDir(proj_path, 0)
		e.Branch = getYAMLString(v, "branch")
		log.Printf("Setting branch => %s \n", e.Branch)
		client.Create(proj_path+"branch", e.Branch, 0)
		e.RepoPath = getYAMLString(v, "repo_path")
		log.Printf("Setting repo_path => %s \n", e.RepoPath)
		client.Create(proj_path+"repo_path", e.Branch, 0)
		e.Deploy = getYAMLString(v, "deploy")
		log.Printf("Setting deploy => %s \n", e.Deploy)
		client.Create(proj_path+"deploy", e.Deploy, 0)
		for _, host := range v.(yaml.Map)["hosts"].(yaml.List) {
			h := Host{URI: host.(yaml.Scalar).String()}
			log.Printf("Setting Hosts => %s \n", proj_path+h.URI)
			client.CreateDir(proj_path+h.URI, 0)
		}
	}
	return e
}

// parseYAML parses the config.yml file and returns the appropriate structs and strings.
func YAMLtoETCD(client *etcd.Client) (c config, err error) {
	config, err := yaml.ReadFile(*ConfigFile)
	if err != nil {
		return c, err
	}
	//client.Create("deploy_user", deployUser, 0)
	//log.Printf("Setting deploy_user => %s \n", deployUser)
	configRoot, _ := config.Root.(yaml.Map)
	projects, _ := configRoot["projects"].(yaml.List)
	for _, p := range projects {
		for k, v := range p.(yaml.Map) {
			log.Printf("Setting project => %s \n", k)
			client.CreateDir(k, 0)
			project_path := "/projects/" + k + "/"
			name := getYAMLString(v, "project_name")
			log.Printf("Setting project name=> %s \n", name)
			client.Create(project_path+"project_name", name, 0)
			repoOwner := getYAMLString(v, "repo_owner")
			log.Printf("Setting repo owner=> %s \n", repoOwner)
			client.Create(project_path+"repo_owner", repoOwner, 0)
			repoName := getYAMLString(v, "repo_name")
			log.Printf("Setting repo owner=> %s \n", repoName)
			client.Create(project_path+"repo_name", repoName, 0)
			for _, v := range v.(yaml.Map)["environments"].(yaml.List) {
				parseYAMLEnvironment(v, client, project_path)
			}
		}
	}
	piv := new(PivotalConfiguration)
	piv.project, _ = config.Get("pivotal_project")
	client.Create("pivotal_project", piv.project, 0)
	log.Printf("Setting pivotal project => %s \n", piv.project)
	piv.token, _ = config.Get("pivotal_token")
	client.Create("piv token", piv.token, 0)
	log.Printf("Setting piv token => %s \n", piv.token)
	notify, _ := config.Get("notify")
	client.Create("notify", notify, 0)
	log.Printf("Setting notify => %s \n", notify)

	return c, err
}

func main() {
	flag.Parse()
	log.Printf("Reading Config file: %s Connecting to ETCD server: %s", *ConfigFile, *ETCDServer)
	// Note the ETCD client library swallows errors connecting to etcd (worry)
	a := etcd.NewClient([]string{*ETCDServer})
	_, err := YAMLtoETCD(a)
	if err != nil {
		fmt.Printf("Failed to Parse Yaml  [%s]\n", err)
	}
}
