package main

// This is a simple quick script to take a goship config file and put into ETCD. Note: It does not wipe out your
// existing etcd setup.

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/coreos/go-etcd/etcd"
	"github.com/gengo/goship/lib"
	"github.com/kylelemons/go-gypsy/yaml"
)

var (
	ConfigFile = flag.String("c", "config.yml", "Path to data directory (default config.yml)")
	ETCDServer = flag.String("e", "http://127.0.0.1:4001", "Etcd Server (default http://127.0.0.1:4001")
)

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

		projPath = projPath + "hosts/"
		log.Printf("Creating Host Directory => %s \n", projPath+"hosts/")
		client.CreateDir(projPath, 0)

		for _, host := range v.(yaml.Map)["hosts"].(yaml.List) {
			h := goship.Host{URI: host.(yaml.Scalar).String()}
			log.Printf("Setting Hosts => %s \n", projPath+h.URI)
			client.CreateDir(projPath+h.URI, 0)
		}
	}
}

// parseYAML parses the config.yml file and returns the appropriate structs and strings.
func YAMLtoETCD(client *etcd.Client) (c goship.Config, err error) {
	config, err := yaml.ReadFile(*ConfigFile)
	if err != nil {
		return c, err
	}
	log.Printf("Setting project root => /projects")
	client.CreateDir("/projects", 0)
	configRoot, _ := config.Root.(yaml.Map)
	projects, _ := configRoot["projects"].(yaml.List)
	for _, p := range projects {
		for k, v := range p.(yaml.Map) {

			projectPath := "/projects/" + k + "/"

			log.Printf("Setting project => %s \n", projectPath)
			client.CreateDir(projectPath, 0)

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

	pivProject, _ := config.Get("pivotal_project")
	setETCD(client, "pivotal_project", pivProject)

	pivToken, _ := config.Get("pivotal_token")
	setETCD(client, "pivotal_token", pivToken)

	deployUser, _ := config.Get("deploy_user")
	setETCD(client, "deploy_user", deployUser)

	goshipHost, _ := config.Get("goship_host")
	setETCD(client, "goship_host", goshipHost)

	notify, _ := config.Get("notify")
	setETCD(client, "notify", notify)

	return c, err
}

func main() {
	flag.Parse()
	log.Printf("Reading Config file: %s Connecting to ETCD server: %s", *ConfigFile, *ETCDServer)
	a := etcd.NewClient([]string{*ETCDServer})
	_, err := YAMLtoETCD(a)
	if err != nil {
		fmt.Printf("Failed to Parse Yaml and Add to ETCD [%s]\n", err)
	}
}
