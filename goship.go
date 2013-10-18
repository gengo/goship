package main

import (
	"bufio"
	"code.google.com/p/go.crypto/ssh"
	"code.google.com/p/go.net/websocket"
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
	"strings"
	"sync"
	"time"
)

var (
	port       = flag.String("p", "8000", "Port number (default 8000)")
	sshPort    = "22"
	configFile = flag.String("c", "config.yml", "Config file (default ./config.yml)")
	keyPath    = flag.String("k", "id_rsa", "Path to private SSH key (default id_rsa)")
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

type Organization struct {
	github.Organization
	Repositories []Repository
}

type Repository struct {
	github.Repository
	PullRequests []github.PullRequest
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
	privateKey := string(getPrivateKey(*keyPath))
	output := remoteCmdOutput(username, hostname, privateKey, fmt.Sprintf("git --git-dir=%s rev-parse HEAD", e.RepoPath))

	return output
}

func getYAMLString(n yaml.Node, key string) string {
	return strings.TrimSpace(n.(yaml.Map)[key].(yaml.Scalar).String())
}

func parseYAMLEnvironment(m yaml.Node) Environment {
	e := Environment{}
	for k, v := range m.(yaml.Map) {
		e.Name = k
		e.Branch = getYAMLString(v, "branch")
		e.RepoPath = getYAMLString(v, "repo_path")
		e.Deploy = getYAMLString(v, "deploy")
		for _, host := range v.(yaml.Map)["hosts"].(yaml.List) {
			h := Host{URI: host.(yaml.Scalar).String()}
			e.Hosts = append(e.Hosts, h)
		}
	}
	return e
}

func parseYAML() (allProjects []Project, deployUser string, orgs *[]string) {
	config, err := yaml.ReadFile(*configFile)
	if err != nil {
		log.Fatal(err)
	}
	deployUser, err = config.Get("deploy_user")
	if err != nil {
		log.Fatal("config.yml is missing deploy_user: " + err.Error())
	}
	configRoot, _ := config.Root.(yaml.Map)
	yamlOrgs := configRoot["orgs"]
	allOrgs := []string{}
	if yamlOrgs != nil {
		for _, o := range yamlOrgs.(yaml.List) {
			allOrgs = append(allOrgs, o.(yaml.Scalar).String())
		}
	}
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
	return allProjects, deployUser, &allOrgs
}

func getCommit(wg *sync.WaitGroup, project Project, env Environment, host Host, deployUser string, i, j int) {
	defer wg.Done()
	lc := string(latestDeployedCommit(deployUser, host.URI+":"+sshPort, env))
	host.LatestCommit = strings.TrimSpace(lc)
	project.Environments[i].Hosts[j] = host
}

//  Get the most recent commit hash on a given branch from GitHub
func getLatestGitHubCommit(wg *sync.WaitGroup, project Project, environment Environment, c *github.Client, repoOwner, repoName string, i int) {
	defer wg.Done()
	opts := &github.CommitsListOptions{SHA: environment.Branch}
	commits, _, err := c.Repositories.ListCommits(repoOwner, repoName, opts)
	if err != nil {
		log.Println("Failed to get commits from GitHub: ", err)
		environment.LatestGitHubCommit = ""
	} else {
		environment.LatestGitHubCommit = *commits[0].SHA
	}
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
			project.Environments[i] = e
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

func insertDeployLogEntry(db sql.DB, environment, diffUrl, user string, success bool) {
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
	db, err := sql.Open("sqlite3", "./deploy_log.db")
	if err != nil {
		log.Fatal("Error opening sqlite db to write to deploy log: " + err.Error())
	}
	defer db.Close()
	q := fmt.Sprintf("select diff_url, user, timestamp, success from logs where environment = \"%s\"", environment)
	rows, err := db.Query(q)
	if err != nil {
		log.Fatal(err)
	}
	type Deploy struct {
		DiffUrl   string
		User      string
		Timestamp time.Time
		Success   bool
	}
	deployments := []Deploy{}
	defer rows.Close()
	for rows.Next() {
		d := Deploy{}
		var success bool
		var diffUrl, user string
		var timestamp time.Time
		rows.Scan(&diffUrl, &user, &timestamp, &success)
		d.DiffUrl = diffUrl
		d.User = user
		d.Timestamp = timestamp
		d.Success = success
		deployments = append(deployments, d)
	}
	// Create and parse Template
	t, err := template.New("deploy_log.html").ParseFiles("templates/deploy_log.html")
	if err != nil {
		log.Panic(err)
	}
	// Render the template
	err = t.Execute(w, map[string]interface{}{"Deployments": deployments})
	if err != nil {
		log.Panic(err)
	}
}

func ProjCommitsHandler(w http.ResponseWriter, r *http.Request) {
	projects, deployUser, _ := parseYAML()
	vars := mux.Vars(r)
	projName := vars["project"]
	proj := getProjectFromName(projects, projName)

	p := retrieveCommits(*proj, deployUser)

	// Render the template
	j, err := json.Marshal(p)
	if err != nil {
		log.Panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(j)
	if err != nil {
		log.Panic(err)
	}
}

type connection struct {
	// The websocket connection.
	ws *websocket.Conn

	// Buffered channel of outbound messages.
	send chan string
}

func (c *connection) writer() {
	for message := range c.send {
		err := websocket.Message.Send(c.ws, message)
		if err != nil {
			break
		}
	}
	c.ws.Close()
}

type hub struct {
	// Registered connections.
	connections map[*connection]bool

	// Inbound messages from the connections.
	broadcast chan string

	// Register requests from the connections.
	register chan *connection

	// Unregister requests from connections.
	unregister chan *connection
}

func (h *hub) run() {
	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
		case c := <-h.unregister:
			delete(h.connections, c)
			close(c.send)
		case m := <-h.broadcast:
			for c := range h.connections {
				select {
				case c.send <- m:
				default:
					delete(h.connections, c)
					close(c.send)
					go c.ws.Close()
				}
			}
		}
	}
}

func (c *connection) reader() {
	for {
		var message string
		err := websocket.Message.Receive(c.ws, &message)
		if err != nil {
			break
		}
		h.broadcast <- message
	}
	c.ws.Close()
}

var h = hub{
	broadcast:   make(chan string),
	register:    make(chan *connection),
	unregister:  make(chan *connection),
	connections: make(map[*connection]bool),
}

func websocketHandler(ws *websocket.Conn) {
	c := &connection{send: make(chan string, 256), ws: ws}
	h.register <- c
	defer func() { h.unregister <- c }()
	go c.writer()
	c.reader()
}

func DeployHandler(w http.ResponseWriter, r *http.Request) {
	projects, _, _ := parseYAML()
	p := r.FormValue("project")
	env := r.FormValue("environment")
	user := r.FormValue("user")
	diffUrl := r.FormValue("diffUrl")
	db, err := sql.Open("sqlite3", "./deploy_log.db")
	if err != nil {
		log.Fatal("Error opening sqlite db to write to deploy log: " + err.Error())
	}
	defer db.Close()
	success := true
	command := getDeployCommand(projects, p, env)
	cmd := exec.Command(command[0], command[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err = cmd.Start(); err != nil {
		log.Fatal(err)
	}
	if err != nil {
		log.Fatal("Could not get stdout of command:" + err.Error())
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		t := scanner.Text()
		cmdOutput := fmt.Sprintf(`{"project": "%s", "environment": "%s", "stdoutLine": "%s"}`, p, env, t)
		h.broadcast <- cmdOutput
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	err = cmd.Wait()
	if err != nil {
		success = false
		log.Println("Deployment failed: " + err.Error())
	}
	insertDeployLogEntry(*db, fmt.Sprintf("%s-%s", p, env), diffUrl, user, success)
}

func DeployPage(w http.ResponseWriter, r *http.Request) {
	p := r.FormValue("project")
	env := r.FormValue("environment")
	user := r.FormValue("user")
	diffUrl := r.FormValue("diffUrl")
	t, err := template.New("deploy.html").ParseFiles("templates/deploy.html")
	if err != nil {
		log.Panic(err)
	}
	err = t.Execute(w, map[string]interface{}{"Project": p, "Env": env, "User": user, "DiffUrl": diffUrl})
	if err != nil {
		log.Panic(err)
	}
}

func getPull(wg *sync.WaitGroup, c *github.Client, orgName, repoName string, pulls []github.PullRequest, prNumber, i int) {
	defer wg.Done()
	pr, _, err := c.PullRequests.Get(orgName, repoName, prNumber)
	if err != nil {
		fmt.Println(err)
	}
	pulls[i] = *pr
}

func getPullsForRepo(wg *sync.WaitGroup, c *github.Client, orgName string, gitHubRepo github.Repository, repos []Repository, i int) {
	defer wg.Done()
	var prWg sync.WaitGroup
	repo := Repository{}
	repo.Repository = gitHubRepo
	pulls, _, err := c.PullRequests.List(orgName, *gitHubRepo.Name, nil)
	if err != nil {
		log.Println(err)
	}
	for i, pull := range pulls {
		prWg.Add(1)
		go getPull(&prWg, c, orgName, *gitHubRepo.Name, pulls, *pull.Number, i)
	}
	prWg.Wait()
	repo.PullRequests = pulls
	repos[i] = repo
}

func getReposForOrg(c *github.Client, orgName string) []Repository {
	var wg sync.WaitGroup
	page := 0
	opt := &github.RepositoryListByOrgOptions{"", github.ListOptions{Page: page}}
	gitHubRepos, _, err := c.Repositories.ListByOrg(orgName, opt)
	if err != nil {
		log.Fatal(err)
	}
	// GitHub API requests that return multiple items
	// are paginated to 30 items, so call github.RepositoryListByOrgOptions
	// untnil we get them all.
	allGitHubRepos := []github.Repository{}
getAllRepos:
	if len(gitHubRepos) < 30 {
		allGitHubRepos = append(allGitHubRepos, gitHubRepos...)
	} else {
		page = page + 1
		opt := &github.RepositoryListByOrgOptions{"", github.ListOptions{Page: page}}
		gitHubRepos, _, err = c.Repositories.ListByOrg(orgName, opt)
		if err != nil {
			println(err)
		}
		allGitHubRepos = append(allGitHubRepos, gitHubRepos...)
		goto getAllRepos
	}
	repos := make([]Repository, len(allGitHubRepos))
	if err != nil {
		log.Println(err)
	}
	for i, repo := range allGitHubRepos {
		wg.Add(1)
		go getPullsForRepo(&wg, c, orgName, repo, repos, i)
	}
	wg.Wait()
	return repos
}

func getOrgs(c *github.Client, orgNames []string) []Organization {
	orgs := []Organization{}
	for _, o := range orgNames {
		gitHubOrg, _, err := c.Organizations.Get(o)
		if err != nil {
			log.Panic(err)
		}
		repos := getReposForOrg(c, o)
		// Filter out repos that have no PRs
		orgRepos := []Repository{}
		for _, r := range repos {
			if len(r.PullRequests) > 0 {
				orgRepos = append(orgRepos, r)
			}
		}
		org := Organization{}
		org.Organization = *gitHubOrg
		org.Repositories = orgRepos
		orgs = append(orgs, org)
	}
	return orgs
}

func PullRequestsHandler(w http.ResponseWriter, r *http.Request) {
	githubToken := os.Getenv("GITHUB_API_TOKEN")
	t := &oauth.Transport{
		Token: &oauth.Token{AccessToken: githubToken},
	}
	c := github.NewClient(t.Client())
	_, _, orgNames := parseYAML()
	orgs := getOrgs(c, *orgNames)
	// Create and parse Template
	tmpl, err := template.New("pulls.html").ParseFiles("templates/pulls.html")
	if err != nil {
		log.Panic(err)
	}
	// Remove orgs with no open PRs
	for i, o := range orgs {
		if len(o.Repositories) == 0 {
			orgs = append(orgs[:i], orgs[i+1:]...)
		}
	}
	// Render the template
	err = tmpl.Execute(w, map[string]interface{}{"Orgs": orgs})
	if err != nil {
		log.Panic(err)
	}
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	projects, _, orgs := parseYAML()
	showPullsLink := len(*orgs) != 0
	// Create and parse Template
	t, err := template.New("index.html").ParseFiles("templates/index.html")
	if err != nil {
		log.Panic(err)
	}
	// Render the template
	err = t.Execute(w, map[string]interface{}{"Projects": projects, "ShowPullsLink": showPullsLink})
	if err != nil {
		log.Panic(err)
	}
}

func main() {
	createDb()
	flag.Parse()
	r := mux.NewRouter()
	r.HandleFunc("/", HomeHandler)
	go h.run()
	http.Handle("/web_push", websocket.Handler(websocketHandler))
	r.HandleFunc("/deploy", DeployPage)
	r.HandleFunc("/deployLog/{environment}", DeployLogHandler)
	r.HandleFunc("/commits/{project}", ProjCommitsHandler)
	r.HandleFunc("/pulls", PullRequestsHandler)
	r.HandleFunc("/deploy_handler", DeployHandler)
	http.Handle("/", r)
	fmt.Println("Running on localhost:" + *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
