package main

import (
	"code.google.com/p/go.crypto/ssh"
	"code.google.com/p/goauth2/oauth"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/gorilla/mux"
	"github.com/kylelemons/go-gypsy/yaml"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/user"
	"path"
	"reflect"
	"strings"
	"sync"
)

var (
	port       = "8080"
	sshPort    = "22"
	configFile = "config.yml"
)

type Host struct {
	URI          string
	LatestCommit string
}

type Environment struct {
	Name               string
	RepoPath           string
	Hosts              []Host
	Branch             string
	LatestGitHubCommit string
}

type Project struct {
	Name         string
	GitHubURL    string
	RepoName     string
	RepoOwner    string
	Environments []Environment
}

/*
Thanks to Russ Cox: https://groups.google.com/d/msg/golang-nuts/OEdSDgEC7js/iyhU9DW_IKcJ
eq reports whether the first argument is equal to
any of the remaining arguments. (for use in templates)
*/
func eq(args ...interface{}) bool {
	if len(args) == 0 {
		return false
	}
	x := args[0]
	switch x := x.(type) {
	case string, int, int64, byte, float32, float64:
		for _, y := range args[1:] {
			if x == y {
				return true
			}
		}
		return false
	}

	for _, y := range args[1:] {
		if reflect.DeepEqual(x, y) {
			return true
		}
	}
	return false
}

func (h *Host) ShortCommitHash() string {
	return h.LatestCommit[:7]
}

func getPrivateKey(filename string) []byte {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Panic("Failed to open private key file: " + err.Error())
	}
	return content
}

type keychain struct {
	key *rsa.PrivateKey
}

func (k *keychain) Key(i int) (interface{}, error) {
	if i != 0 {
		return nil, nil
	}
	return &k.key.PublicKey, nil
}

func (k *keychain) Sign(i int, rand io.Reader, data []byte) (sig []byte, err error) {
	hashFunc := crypto.SHA1
	h := hashFunc.New()
	h.Write(data)
	digest := h.Sum(nil)
	return rsa.SignPKCS1v15(rand, k.key, hashFunc, digest)
}

func remoteCmdOutput(username, hostname, privateKey, cmd string) []byte {
	block, _ := pem.Decode([]byte(privateKey))
	rsakey, _ := x509.ParsePKCS1PrivateKey(block.Bytes)
	clientKey := &keychain{rsakey}
	clientConfig := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.ClientAuth{
			ssh.ClientAuthKeyring(clientKey),
		},
	}
	client, err := ssh.Dial("tcp", hostname, clientConfig)
	if err != nil {
		log.Panic("Failed to dial: " + err.Error())
	}
	session, err := client.NewSession()
	if err != nil {
		log.Panic("Failed to create session: " + err.Error())
	}
	defer session.Close()
	output, err := session.Output(cmd)
	if err != nil {
		log.Panic("Failed to run cmd: " + err.Error())
	}
	return output
}

func latestDeployedCommit(username, hostname string, e Environment) []byte {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	privateKey := string(getPrivateKey(path.Join(usr.HomeDir, "/.ssh/id_rsa")))
	output := remoteCmdOutput(username, hostname, privateKey, fmt.Sprintf("git --git-dir=%s rev-parse %s", e.RepoPath, e.Branch))

	return output
}

func getYAMLString(n yaml.Node, key string) string {
	return n.(yaml.Map)[key].(yaml.Scalar).String()
}

func parseYAMLEnvironment(m yaml.Node) Environment {
	e := Environment{}
	for k, v := range m.(yaml.Map) {
		e.Name = k
		e.Branch = getYAMLString(v, "branch")
		e.RepoPath = getYAMLString(v, "repo_path")
		for _, v := range v.(yaml.Map)["hosts"].(yaml.List) {
			h := Host{URI: v.(yaml.Scalar).String()}
			e.Hosts = append(e.Hosts, h)
		}
	}
	return e
}

func parseYAML() (allProjects []Project, deployUser string) {
	config, err := yaml.ReadFile(configFile)
	if err != nil {
		log.Fatal(err)
	}
	deployUser, err = config.Get("deploy_user")
	if err != nil {
		log.Fatal("config.yml is missing deploy_user: " + err.Error())
	}
	configRoot, _ := config.Root.(yaml.Map)
	projects, _ := configRoot["projects"].(yaml.List)
	allProjects = []Project{}
	for _, p := range projects {
		for _, v := range p.(yaml.Map) {
			proj := Project{Name: getYAMLString(v, "project_name"),
				GitHubURL: getYAMLString(v, "github_url"),
				RepoName:  getYAMLString(v, "repo_name"),
				RepoOwner: getYAMLString(v, "repo_owner")}
			for _, v := range v.(yaml.Map)["environments"].(yaml.List) {
				proj.Environments = append(proj.Environments, parseYAMLEnvironment(v))
			}
			allProjects = append(allProjects, proj)
		}
	}
	return allProjects, deployUser
}

func getCommit(wg *sync.WaitGroup, projects []Project, env Environment, host Host, deployUser string, i, j, k int) {
	defer wg.Done()
	host.LatestCommit = string(latestDeployedCommit(deployUser, host.URI+":"+sshPort, env))
	host.LatestCommit = strings.Trim(host.LatestCommit, "\n\r")
	projects[i].Environments[j].Hosts[k] = host
}

//  Get the most recent commit hash on a given branch from GitHub
func getLatestGitHubCommit(wg *sync.WaitGroup, projects []Project, environment Environment, c *github.Client, repoOwner, repoName string, i, j int) {
	defer wg.Done()
	opts := &github.CommitsListOptions{SHA: environment.Branch}
	commits, _, err := c.Repositories.ListCommits(repoOwner, repoName, opts)
	if err != nil {
		log.Panic(err)
	}
	environment.LatestGitHubCommit = *commits[0].SHA
	projects[i].Environments[j] = environment
}

func retrieveCommits(projects []Project, deployUser string) []Project {
	// define a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup
	githubToken := os.Getenv("GITHUB_API_TOKEN")
	t := &oauth.Transport{
		Token: &oauth.Token{AccessToken: githubToken},
	}
	client := github.NewClient(t.Client())
	for i, project := range projects {
		for j, environment := range project.Environments {
			for k, host := range environment.Hosts {
				// start a goroutine for SSH-ing on to the machine
				wg.Add(1)
				go getCommit(&wg, projects, environment, host, deployUser, i, j, k)
			}
			wg.Add(1)
			go getLatestGitHubCommit(&wg, projects, environment, client, project.RepoOwner, project.RepoName, i, j)
		}
	}
	// wait for goroutines to finish
	wg.Wait()
	return projects
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	projects, deployUser := parseYAML()
	projects = retrieveCommits(projects, deployUser)
	t, err := template.New("index.html").Funcs(template.FuncMap{"eq": eq}).ParseFiles("templates/index.html")
	if err != nil {
		log.Panic(err)
	}
	err = t.Execute(w, map[string]interface{}{"Projects": projects})
	if err != nil {
		log.Panic(err)
	}
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", HomeHandler)
	fmt.Println("Running on localhost:" + port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
