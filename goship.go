package main

import (
	"bytes"
	"code.google.com/p/go.crypto/ssh"
	"code.google.com/p/goauth2/oauth"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
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
	"os/exec"
	"os/user"
	"path"
	"strings"
	"sync"
)

var (
	port       = flag.String("p", "8000", "Port number (default 8000)")
	sshPort    = "22"
	configFile = "config.yml"
	keyPath    = ".ssh/id_rsa" // The path to your private SSH key. Home directory will be prepended
)

type Host struct {
	URI          string
	LatestCommit string
}

type Environment struct {
	Name               string
	Deploy             string
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

func (h *Host) GitHubCommitURL(p Project) string {
	return fmt.Sprintf("%s/commit/%s", p.GitHubURL, h.LatestCommit)
}

func (h *Host) GitHubDiffURL(p Project, e Environment) *string {
	if h.LatestCommit != e.LatestGitHubCommit {
		s := fmt.Sprintf("%s/compare/%s...%s", p.GitHubURL, h.LatestCommit, e.LatestGitHubCommit)
		return &s
	}
	return nil
}

func (e *Environment) Deployable() bool {
	for _, h := range e.Hosts {
		if e.LatestGitHubCommit != h.LatestCommit {
			return true
		}
	}
	return false
}

func (h *Host) ShortCommitHash() string {
	if len(h.LatestCommit) == 0 {
		return ""
	}
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

func (k *keychain) Key(i int) (ssh.PublicKey, error) {
	if i != 0 {
		return nil, nil
	}
	return ssh.NewRSAPublicKey(&k.key.PublicKey), nil
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
		log.Println("ERROR: Failed to dial: " + err.Error())
	}
	session, err := client.NewSession()
	if err != nil {
		log.Println("ERROR: Failed to create session: " + err.Error())
	}
	defer session.Close()
	output, err := session.Output(cmd)
	if err != nil {
		log.Printf("ERROR: Failed to run cmd on host %s: %s", hostname, err.Error())
	}
	return output
}

func latestDeployedCommit(username, hostname string, e Environment) []byte {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	privateKey := string(getPrivateKey(path.Join(usr.HomeDir, keyPath)))
	output := remoteCmdOutput(username, hostname, privateKey, fmt.Sprintf("git --git-dir=%s rev-parse HEAD", e.RepoPath))

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
		e.Deploy = getYAMLString(v, "deploy")
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
			name := getYAMLString(v, "project_name")
			repoOwner := getYAMLString(v, "repo_owner")
			repoName := getYAMLString(v, "repo_name")
			githubUrl := fmt.Sprintf("https://github.com/%s/%s", repoOwner, repoName)
			proj := Project{Name: name, GitHubURL: githubUrl, RepoName: repoName, RepoOwner: repoOwner}
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
	lc := string(latestDeployedCommit(deployUser, host.URI+":"+sshPort, env))
	host.LatestCommit = strings.Trim(lc, "\n\r")
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

func DeployHandler(w http.ResponseWriter, r *http.Request) {
	var command []string
	projects, _ := parseYAML()
	p := r.FormValue("project")
	env := r.FormValue("environment")
	for i, project := range projects {
		if project.Name == p {
			for j, environment := range project.Environments {
				if environment.Name == env {
					command = strings.Split(projects[i].Environments[j].Deploy, " ")
				}
			}
		}
	}
	var out bytes.Buffer
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	t, err := template.New("deploy.html").ParseFiles("templates/deploy.html")
	if err != nil {
		log.Panic(err)
	}
	// Render the template
	err = t.Execute(w, out.String())
	if err != nil {
		log.Panic(err)
	}
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	projects, deployUser := parseYAML()
	// Get the most recently-deployed commits from each server, as well as the most recent commit from GitHub
	projects = retrieveCommits(projects, deployUser)
	// Create and parse Template
	t, err := template.New("index.html").ParseFiles("templates/index.html")
	if err != nil {
		log.Panic(err)
	}
	// Render the template
	err = t.Execute(w, map[string]interface{}{"Projects": projects})
	if err != nil {
		log.Panic(err)
	}
}

func main() {
	flag.Parse()
	r := mux.NewRouter()
	r.HandleFunc("/", HomeHandler)
	r.HandleFunc("/deploy", DeployHandler)
	fmt.Println("Running on localhost:" + *port)
	log.Fatal(http.ListenAndServe(":"+*port, r))
}
