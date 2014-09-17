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

// Send etcd Data and output.
func setETCD(client *etcd.Client, full_key, value string) {
	log.Printf("Setting %s => %s \n", full_key, value)
	client.Create(full_key, value, 0)
}

// parseYAMLEnvironment populates an Environment given a yaml.Node and returns the Environment.
func YAMLtoETCDEnvironment(m yaml.Node, client *etcd.Client, projPath string) {

	for k, v := range m.(yaml.Map) {
		projPath = projPath + "environments/" + k + "/"

		log.Printf("Setting env name=> %s \n", projPath)
		client.CreateDir(projPath, 0)

		branch := getYAMLString(v, "branch")
		setETCD(client, projPath+"branch", branch)

		repoPath := getYAMLString(v, "repo_path")
		setETCD(client, projPath+"repo_path", repoPath)

		deploy := getYAMLString(v, "deploy")
		setETCD(client, projPath+"deploy", deploy)

		for _, host := range v.(yaml.Map)["hosts"].(yaml.List) {
			h := Host{URI: host.(yaml.Scalar).String()}
			log.Printf("Setting Hosts => %s \n", projPath+h.URI)
			client.CreateDir(projPath+h.URI, 0)
		}
	}
}

// parseYAML parses the config.yml file and returns the appropriate structs and strings.
func YAMLtoETCD(client *etcd.Client) (c config, err error) {
	config, err := yaml.ReadFile(*ConfigFile)
	if err != nil {
		return c, err
	}
	configRoot, _ := config.Root.(yaml.Map)
	projects, _ := configRoot["projects"].(yaml.List)
	for _, p := range projects {
		for k, v := range p.(yaml.Map) {

			log.Printf("Setting project => %s \n", k)
			client.CreateDir(k, 0)

			projectPath := "/projects/" + k + "/"

			name := getYAMLString(v, "project_name")
			setETCD(client, projectPath+"project_name", name)

			repoOwner := getYAMLString(v, "repo_owner")
			setETCD(client, projectPath+"repo_owner", repoOwner)

			repoName := getYAMLString(v, "repo_name")
			setETCD(client, projectPath+"repo_name", repoName)

			for _, v := range v.(yaml.Map)["environments"].(yaml.List) {
				YAMLtoETCDEnvironment(v, client, projectPath)
			}

		}
	}

	piv_project, _ := config.Get("pivotal_project")
	setETCD(client, "pivotal_project", piv_project)

	piv_token, _ := config.Get("pivotal_token")
	setETCD(client, "piv_token", piv_token)

	notify, _ := config.Get("notify")
	setETCD(client, "notify", notify)

	return c, err
}

func main() {
	flag.Parse()
	log.Printf("Reading Config file: %s Connecting to ETCD server: %s", *ConfigFile, *ETCDServer)
	// Note the ETCD client library swallows errors connecting to etcd (worry)
	a := etcd.NewClient([]string{*ETCDServer})
	_, err := YAMLtoETCD(a)
	if err != nil {
		fmt.Printf("Failed to Parse Yaml and Add to ETCD [%s]\n", err)
	}
}
