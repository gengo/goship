package main

import (
	"bytes"
	"code.google.com/p/go.crypto/ssh"
	"code.google.com/p/goauth2/oauth"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/gorilla/mux"
	"github.com/kylelemons/go-gypsy/yaml"
	_ "github.com/mattn/go-sqlite3"
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
	URI             string
	LatestCommit    string
	GitHubCommitURL string
	GitHubDiffURL   *string
	ShortCommitHash string
}

type Environment struct {
	Name               string
	Deploy             string
	RepoPath           string
	Hosts              []Host
	Branch             string
	LatestGitHubCommit string
	IsDeployable       bool
}

type Project struct {
	Name         string
	GitHubURL    string
	RepoName     string
	RepoOwner    string
	Environments []Environment
}

func (h *Host) GetGitHubCommitURL(p Project) string {
	return fmt.Sprintf("%s/commit/%s", p.GitHubURL, h.LatestCommit)
}

func (h *Host) GetGitHubDiffURL(p Project, e Environment) *string {
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

func (h *Host) GetShortCommitHash() string {
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
	pubkey, err := ssh.NewPublicKey(&k.key.PublicKey)
	if err != nil {
		log.Panic(err)
	}
	return pubkey, nil
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
		return []byte{}
	}
	session, err := client.NewSession()
	if err != nil {
		log.Println("ERROR: Failed to create session: " + err.Error())
		return []byte{}
	}
	defer session.Close()
	output, err := session.Output(cmd)
	if err != nil {
		log.Printf("ERROR: Failed to run cmd on host %s: %s", hostname, err.Error())
		return []byte{}
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

func getCommit(wg *sync.WaitGroup, project Project, env Environment, host Host, deployUser string, i, j int) {
	defer wg.Done()
	lc := string(latestDeployedCommit(deployUser, host.URI+":"+sshPort, env))
	host.LatestCommit = strings.Trim(lc, "\n\r")
	project.Environments[i].Hosts[j] = host
}

//  Get the most recent commit hash on a given branch from GitHub
func getLatestGitHubCommit(wg *sync.WaitGroup, project Project, environment Environment, c *github.Client, repoOwner, repoName string, i int) {
	defer wg.Done()
	opts := &github.CommitsListOptions{SHA: environment.Branch}
	commits, _, err := c.Repositories.ListCommits(repoOwner, repoName, opts)
	if err != nil {
		log.Panic(err)
	}
	environment.LatestGitHubCommit = *commits[0].SHA
	project.Environments[i] = environment
}

func retrieveCommits(project Project, deployUser string) Project {
	// define a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup
	githubToken := os.Getenv("GITHUB_API_TOKEN")
	t := &oauth.Transport{
		Token: &oauth.Token{AccessToken: githubToken},
	}
	client := github.NewClient(t.Client())
	for i, environment := range project.Environments {
		for j, host := range environment.Hosts {
			// start a goroutine for SSHing on to the machine
			wg.Add(1)
			go getCommit(&wg, project, environment, host, deployUser, i, j)
		}
		wg.Add(1)
		go getLatestGitHubCommit(&wg, project, environment, client, project.RepoOwner, project.RepoName, i)
	}
	// wait for goroutines to finish
	wg.Wait()
	for i, e := range project.Environments {
		if e.Deployable() {
			e.IsDeployable = true
		}
		for j, host := range e.Hosts {
			host.GitHubCommitURL = host.GetGitHubCommitURL(project)
			host.GitHubDiffURL = host.GetGitHubDiffURL(project, e)
			host.ShortCommitHash = host.GetShortCommitHash()
			project.Environments[i].Hosts[j] = host
		}
	}
	return project
}

func insertDeployLogEntry(db sql.DB, environment, diffUrl, user string, success int) {
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Prepare("insert into logs(environment, diff_url, user, success) values(?, ?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()
	_, err = stmt.Exec(environment, diffUrl, user, success)
	if err != nil {
		log.Fatal(err)
	}
	tx.Commit()
}

func getProjectFromName(projects []Project, projectName string) *Project {
	for _, project := range projects {
		if project.Name == projectName {
			return &project
		}
	}
	return nil
}

func getEnvironmentFromName(projects []Project, projectName, environmentName string) *Environment {
	p := getProjectFromName(projects, projectName)
	for _, environment := range p.Environments {
		if environment.Name == environmentName {
			return &environment
		}
	}
	return nil
}

func getDeployCommand(projects []Project, projectName, environmentName string) []string {
	var command []string
	e := getEnvironmentFromName(projects, projectName, environmentName)
	command = strings.Split(e.Deploy, " ")
	return command
}

func createDb() {
	db, err := sql.Open("sqlite3", "./deploy_log.db")
	if err != nil {
		log.Fatal("Error opening or creating deploy_log.db: " + err.Error())
	}
	defer db.Close()
	sql := `create table if not exists logs (id integer not null primary key autoincrement, environment text, diff_url text, user text, timestamp datetime default current_timestamp, success boolean);`
	_, err = db.Exec(sql)
	if err != nil {
		log.Fatal("Error creating logs table: " + err.Error())
	}
}

func DeployLogHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	environment := vars["environment"]
	fmt.Println(environment)
}

func ProjCommitsHandler(w http.ResponseWriter, r *http.Request) {
	projects, deployUser := parseYAML()
	vars := mux.Vars(r)
	projName := vars["project"]
	proj := getProjectFromName(projects, projName)
	p := retrieveCommits(*proj, deployUser)
	t, err := template.New("project.html").ParseFiles("templates/project.html")
	if err != nil {
		log.Panic(err)
	}
	// Render the template
	j, err := json.Marshal(p)
	if err != nil {
		log.Panic(err)
	}
	err = t.Execute(w, string(j))
	if err != nil {
		log.Panic(err)
	}
}

func DeployHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "./deploy_log.db")
	if err != nil {
		log.Fatal("Error opening sqlite db to write to deploy log: " + err.Error())
	}
	defer db.Close()
	projects, _ := parseYAML()
	p := r.FormValue("project")
	env := r.FormValue("environment")
	user := r.FormValue("user")
	diffUrl := r.FormValue("diffUrl")
	success := 1
	command := getDeployCommand(projects, p, env)
	var out bytes.Buffer
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		success = 0
		log.Println("Deployment failed: " + err.Error())
	}
	insertDeployLogEntry(*db, fmt.Sprintf("%s-%s", p, env), diffUrl, user, success)
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
	//projects, deployUser := parseYAML()
	projects, _ := parseYAML()
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
	createDb()
	flag.Parse()
	r := mux.NewRouter()
	r.HandleFunc("/", HomeHandler)
	r.HandleFunc("/deploy", DeployHandler)
	r.HandleFunc("/deployLog/{environment}", DeployLogHandler)
	r.HandleFunc("/commits/{project}", ProjCommitsHandler)
	fmt.Println("Running on localhost:" + *port)
	log.Fatal(http.ListenAndServe(":"+*port, r))
}
