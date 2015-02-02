package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"code.google.com/p/go.crypto/ssh"
	"code.google.com/p/go.net/websocket"
	"code.google.com/p/goauth2/oauth"
	"github.com/coreos/go-etcd/etcd"
	"github.com/gengo/goship/lib"
	"github.com/google/go-github/github"
	"github.com/gorilla/sessions"
	"github.com/stretchr/gomniauth"
	githubOauth "github.com/stretchr/gomniauth/providers/github"
	"github.com/stretchr/objx"
)

var (
	bindAddress = flag.String("b", "localhost:8000", "Address to bind (default localhost:8000)")
	sshPort     = "22"
	keyPath     = flag.String("k", "id_rsa", "Path to private SSH key (default id_rsa)")
	dataPath    = flag.String("d", "data/", "Path to data directory (default ./data/)")
	ETCDServer  = flag.String("e", "http://127.0.0.1:4001", "Etcd Server (default http://127.0.0.1:4001")
)

var store = sessions.NewCookieStore([]byte("jhjhjhjhjhjjhjhhj"))
var session_name = "goship"

var authentication auth

// gitHubPaginationLimit is the default pagination limit for requests to the GitHub API that return multiple items.
const (
	gitHubPaginationLimit = 30
	pivotalCommentURL     = "https://www.pivotaltracker.com/services/v5/projects/%s/stories/%s/comments"
	gitHubAPITokenEnvVar  = "GITHUB_API_TOKEN"
)

func diffURL(owner, repoName, fromRevision, toRevision string) string {
	return fmt.Sprintf("https://github.com/%s/%s/compare/%s...%s", owner, repoName, fromRevision, toRevision)
}

// remoteCmdOutput runs the given command on a remote server at the given hostname as the given user.
func remoteCmdOutput(username, hostname, cmd string, privateKey []byte) (b []byte, err error) {
	p, err := ssh.ParseRawPrivateKey(privateKey)
	if err != nil {
		return b, err
	}
	s, err := ssh.NewSignerFromKey(p)
	if err != nil {
		return b, err
	}
	pub := ssh.PublicKeys(s)
	clientConfig := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{pub},
	}
	client, err := ssh.Dial("tcp", hostname, clientConfig)
	if err != nil {
		return b, errors.New("ERROR: Failed to dial: " + err.Error())
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return b, errors.New("ERROR: Failed to create session: " + err.Error())
	}
	defer session.Close()
	b, err = session.Output(cmd)
	if err != nil {
		return b, fmt.Errorf("ERROR: Failed to run cmd on host %s: %s", hostname, err.Error())
	}
	return b, nil
}

// latestDeployedCommit gets the latest commit hash on the host.
func latestDeployedCommit(username, hostname string, e goship.Environment) (b []byte, err error) {
	p, err := ioutil.ReadFile(*keyPath)
	if err != nil {
		return b, errors.New("Failed to open private key file: " + err.Error())
	}
	o, err := remoteCmdOutput(username, hostname, fmt.Sprintf("git --git-dir=%s rev-parse HEAD", e.RepoPath), p)
	if err != nil {
		return b, err
	}
	return o, nil
}

// getCommit is called in a goroutine and gets the latest deployed commit on a host.
// It updates the Environment in-place.
func getCommit(wg *sync.WaitGroup, project goship.Project, env goship.Environment, host goship.Host, deployUser string, i, j int) {
	defer wg.Done()
	lc, err := latestDeployedCommit(deployUser, host.URI+":"+sshPort, env)
	if err != nil {
		log.Printf("ERROR: failed to get latest deployed commit: %s, %s. Error: %v", host.URI, deployUser, err)
		host.LatestCommit = string(lc)
		project.Environments[i].Hosts[j] = host
	}
	host.LatestCommit = strings.TrimSpace(string(lc))
	project.Environments[i].Hosts[j] = host
}

// getLatestGitHubCommit is called in a goroutine and retrieves the latest commit
// from GitHub for a given branch of a project. It updates the Environment in-place.
func getLatestGitHubCommit(wg *sync.WaitGroup, project goship.Project, environment goship.Environment, c *github.Client, repoOwner, repoName string, i int) {
	defer wg.Done()
	opts := &github.CommitsListOptions{SHA: environment.Branch}
	commits, _, err := c.Repositories.ListCommits(repoOwner, repoName, opts)
	if err != nil {
		log.Println("ERROR: Failed to get commits from GitHub: ", err)
		environment.LatestGitHubCommit = ""
	} else {
		environment.LatestGitHubCommit = *commits[0].SHA
	}
	project.Environments[i] = environment
}

// retrieveCommits fetches the latest deployed commits as well
// as the latest GitHub commits for a given Project.
func retrieveCommits(project goship.Project, deployUser string) goship.Project {
	// define a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup
	githubToken := os.Getenv(gitHubAPITokenEnvVar)
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
		e.IsDeployable = e.Deployable()
		project.Environments[i] = e
		for j, host := range e.Hosts {
			host.GitHubCommitURL = host.LatestGitHubCommitURL(project)
			host.GitHubDiffURL = host.LatestGitHubDiffURL(project, e)
			host.ShortCommitHash = host.LatestShortCommitHash()
			project.Environments[i].Hosts[j] = host
		}
	}
	return project
}

type DeployLogEntry struct {
	DiffURL       string
	ToRevisionMsg string
	User          string
	Success       bool
	Time          time.Time
	FormattedTime string `json:",omitempty"`
}

type ByTime []DeployLogEntry

func (d ByTime) Len() int           { return len(d) }
func (d ByTime) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
func (d ByTime) Less(i, j int) bool { return d[i].Time.After(d[j].Time) }

func writeJSON(d []DeployLogEntry, file string) error {
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(file, b, 0755)
}

func readEntries(env string) ([]DeployLogEntry, error) {
	var d []DeployLogEntry
	b, err := ioutil.ReadFile(path.Join(*dataPath, env+".json"))
	if err != nil {
		return d, err
	}
	if len(b) == 0 {
		log.Printf("No deploy logs found for: %s", env)
		return []DeployLogEntry{}, nil
	}
	err = json.Unmarshal(b, &d)
	if err != nil {
		return d, err
	}

	return d, nil
}

func prepareDataFiles(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		_, err := os.Create(path)
		if err != nil {
			return err
		}
		err = writeJSON([]DeployLogEntry{}, path)
		if err != nil {
			return err
		}
	}

	return nil
}

func insertEntry(env, owner, repoName, fromRevision, toRevision, user string, success bool, time time.Time) error {
	path := path.Join(*dataPath, env+".json")
	err := prepareDataFiles(path)
	if err != nil {
		return err
	}

	e, err := readEntries(env)
	if err != nil {
		return err
	}
	gt := os.Getenv(gitHubAPITokenEnvVar)
	t := &oauth.Transport{
		Token: &oauth.Token{AccessToken: gt},
	}
	c := github.NewClient(t.Client())
	com, _, err := c.Git.GetCommit(owner, repoName, toRevision)
	if err != nil {
		log.Println("Error getting commit msg: ", err)
	}
	var m string
	if com.Message != nil {
		m = *com.Message
	}
	diffURL := diffURL(owner, repoName, fromRevision, toRevision)
	d := DeployLogEntry{DiffURL: diffURL, ToRevisionMsg: m, User: user, Time: time, Success: success}
	e = append(e, d)
	err = writeJSON(e, path)
	if err != nil {
		return err
	}
	return nil
}

func appendDeployOutput(env string, output string, timestamp time.Time) {
	logDir := path.Join(*dataPath, env)
	path := path.Join(logDir, timestamp.String()+".log")

	if _, err := os.Stat(logDir); err != nil {
		if os.IsNotExist(err) {
			err := os.Mkdir(logDir, 0755)
			if err != nil {
				fmt.Printf("ERROR: %s", err)
			}
		}
	}

	out, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		fmt.Printf("ERROR: %s", err)
	}

	defer out.Close()

	io.WriteString(out, output+"\n")
}

// getDeployCommand returns the deployment command for a given
// environment as a string slice that has been split on spaces.
func getDeployCommand(projects []goship.Project, projectName, environmentName string) (s []string, err error) {
	e, err := goship.EnvironmentFromName(projects, projectName, environmentName)
	if err != nil {
		return s, err
	}

	return strings.Split(e.Deploy, " "), nil
}

func formatTime(t time.Time) string {
	s := time.Since(t)
	switch {
	case s.Seconds() < 60:
		f := "second"
		if math.Floor(s.Seconds()) > 1 {
			f += "s"
		}
		return fmt.Sprintf("%d "+f+" ago", int(s.Seconds()))
	case s.Minutes() < 60:
		f := "minute"
		if math.Floor(s.Minutes()) > 1 {
			f += "s"
		}
		return fmt.Sprintf("%d "+f+" ago", int(s.Minutes()))
	case s.Hours() < 24:
		f := "hour"
		if math.Floor(s.Hours()) > 1 {
			f += "s"
		}
		return fmt.Sprintf("%d "+f+" ago", int(s.Hours()))
	default:
		layout := "Jan 2, 2006 at 3:04pm (MST)"
		return t.Format(layout)
	}
}

func DeployOutputHandler(w http.ResponseWriter, r *http.Request, env string, formattedTime string) {
	log := path.Join(*dataPath, env, formattedTime+".log")

	b, err := ioutil.ReadFile(log)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Write(b)
}

func DeployLogHandler(w http.ResponseWriter, r *http.Request, full_env string, environment goship.Environment) {
	u, _ := getUser(r)
	d, err := readEntries(full_env)
	if err != nil {
		log.Println("Error: ", err)
	}
	t, err := template.New("deploy_log.html").ParseFiles("templates/deploy_log.html", "templates/base.html")
	if err != nil {
		log.Println("ERROR: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for i := range d {
		d[i].FormattedTime = formatTime(d[i].Time)
	}
	sort.Sort(ByTime(d))
	t.ExecuteTemplate(w, "base", map[string]interface{}{"Deployments": d, "User": u, "Env": full_env, "Environment": environment})
}

func ProjCommitsHandler(w http.ResponseWriter, r *http.Request, projName string) {
	c, err := goship.ParseETCD(etcd.NewClient([]string{*ETCDServer}))
	if err != nil {
		log.Println("ERROR: Parsing etc ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	proj, err := goship.ProjectFromName(c.Projects, projName)
	if err != nil {
		log.Println("ERROR:  Getting Project from name", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p := retrieveCommits(*proj, c.DeployUser)

	j, err := json.Marshal(p)
	if err != nil {
		log.Println("ERROR: Retrieving Commits ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(j)
	if err != nil {
		log.Println("ERROR: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

func sendOutput(wg *sync.WaitGroup, scanner *bufio.Scanner, p, e string, deployTime time.Time) {
	defer wg.Done()
	for scanner.Scan() {
		t := scanner.Text()
		msg := struct {
			Project     string
			Environment string
			StdoutLine  string
		}{p, e, stripANSICodes(strings.TrimSpace(t))}
		cmdOutput, err := json.Marshal(msg)
		if err != nil {
			log.Println("ERROR marshalling JSON: ", err.Error())
		}
		h.broadcast <- string(cmdOutput)

		go appendDeployOutput(fmt.Sprintf("%s-%s", p, e), t, deployTime)
	}
	if err := scanner.Err(); err != nil {
		log.Println("Error reading command output: " + err.Error())
		return
	}
}

func stripANSICodes(t string) string {
	ansi := regexp.MustCompile(`\x1B\[[0-9;]{1,4}[mK]`)
	return ansi.ReplaceAllString(t, "")
}

func notify(n, msg string) error {
	cmd := exec.Command(n, msg)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func startNotify(n, user, p, env string) error {
	msg := fmt.Sprintf("%s is deploying %s to %s.", user, p, env)
	err := notify(n, msg)
	if err != nil {
		return err
	}
	return nil
}

func endNotify(n, p, env string, success bool) error {
	msg := fmt.Sprintf("%s successfully deployed to %s.", p, env)
	if !success {
		msg = fmt.Sprintf("%s deployment to %s failed.", p, env)
	}
	err := notify(n, msg)
	if err != nil {
		return err
	}
	return nil
}

func DeployHandler(w http.ResponseWriter, r *http.Request) {
	c, err := goship.ParseETCD(etcd.NewClient([]string{*ETCDServer}))
	if err != nil {
		log.Println("ERROR: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p := r.FormValue("project")
	env := r.FormValue("environment")
	u, err := getUser(r)
	if err != nil {
		log.Fatal("Failed to get a user while deploying!")
	}
	user := u.UserName
	fromRevision := r.FormValue("from_revision")
	toRevision := r.FormValue("to_revision")
	owner := r.FormValue("repo_owner")
	name := r.FormValue("repo_name")
	if c.Notify != "" {
		err := startNotify(c.Notify, user, p, env)
		if err != nil {
			log.Println("Error: ", err.Error())
		}
	}

	deployTime := time.Now()
	success := true
	command, err := getDeployCommand(c.Projects, p, env)
	if err != nil {
		log.Println("ERROR: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cmd := exec.Command(command[0], command[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println("ERROR: could not get stdout of command:" + err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Println("ERROR: could not get stderr of command:" + err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err = cmd.Start(); err != nil {
		log.Println("ERROR: could not run deployment command: " + err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go sendOutput(&wg, bufio.NewScanner(stdout), p, env, deployTime)
	go sendOutput(&wg, bufio.NewScanner(stderr), p, env, deployTime)
	wg.Wait()

	err = cmd.Wait()
	if err != nil {
		success = false
		log.Println("Deployment failed: " + err.Error())
	}
	if c.Notify != "" {
		err = endNotify(c.Notify, p, env, success)
		if err != nil {
			log.Println("Error: ", err.Error())
		}
	}

	if (c.Pivotal.Token != "") && (c.Pivotal.Project != "") && success {
		err := postToPivotal(c.Pivotal, env, owner, name, toRevision, fromRevision)
		if err != nil {
			log.Println("ERROR: ", err)
		} else {
			log.Printf("Pivotal Info: %s %s", c.Pivotal.Token, c.Pivotal.Project)
		}
	}

	err = insertEntry(fmt.Sprintf("%s-%s", p, env), owner, name, fromRevision, toRevision, user, success, deployTime)
	if err != nil {
		log.Println("ERROR: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func postToPivotal(piv *goship.PivotalConfiguration, env, owner, name, latest, current string) error {
	gt := os.Getenv(gitHubAPITokenEnvVar)
	t := &oauth.Transport{
		Token: &oauth.Token{AccessToken: gt},
	}
	c := github.NewClient(t.Client())
	comp, _, err := c.Repositories.CompareCommits(owner, name, current, latest)
	if err != nil {
		return err
	}
	pivRE, err := regexp.Compile("\\[.*#(\\d+)\\].*")
	if err != nil {
		return err
	}
	s := map[string]bool{}
	layout := "2006-01-02 15:04:05"
	timestamp := time.Now()
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		layout += " (UTC)"
		log.Println("error: time zone information for Asia/Tokyo not found")
	} else {
		layout += " (JST)"
		timestamp = timestamp.In(loc)
	}
	for _, commit := range comp.Commits {
		cmi := *commit.Commit
		cm := *cmi.Message
		ids := pivRE.FindStringSubmatch(cm)
		if ids != nil {
			id := ids[1]
			_, exists := s[id]
			if !exists {
				s[id] = true
				m := fmt.Sprintf("Deployed to %s: %s", env, timestamp.Format(layout))
				go PostPivotalComment(id, m, piv)
			}
		}
	}
	return nil
}

func PostPivotalComment(id string, m string, piv *goship.PivotalConfiguration) (err error) {
	p := url.Values{}
	p.Set("text", m)
	req, err := http.NewRequest("POST", fmt.Sprintf(pivotalCommentURL, piv.Project, id), nil)
	if err != nil {
		log.Println("ERROR: could not form put request to Pivotal: ", err)
		return err
	}
	req.URL.RawQuery = p.Encode()
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-TrackerToken", piv.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("ERROR: could not make put request to Pivotal: ", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Println("ERROR: non-200 Response from Pivotal API: ", resp.Status)
	}
	return nil
}

func DeployPage(w http.ResponseWriter, r *http.Request) {
	p := r.FormValue("project")
	env := r.FormValue("environment")
	user, _ := getUser(r)
	fromRevision := r.FormValue("from_revision")
	toRevision := r.FormValue("to_revision")
	repo_owner := r.FormValue("repo_owner")
	repo_name := r.FormValue("repo_name")
	t, err := template.New("deploy.html").ParseFiles("templates/deploy.html", "templates/base.html")
	if err != nil {
		log.Println("ERROR: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	t.ExecuteTemplate(w, "base", map[string]interface{}{"Project": p, "Env": env, "User": user, "BindAddress": bindAddress, "RepoOwner": repo_owner, "RepoName": repo_name, "ToRevision": toRevision, "FromRevision": fromRevision})
}

// Sort interface for sorting projects
type ByName []goship.Project

func (slice ByName) Len() int           { return len(slice) }
func (slice ByName) Less(i, j int) bool { return slice[i].Name < slice[j].Name }
func (slice ByName) Swap(i, j int)      { slice[i], slice[j] = slice[j], slice[i] }

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	c, err := goship.ParseETCD(etcd.NewClient([]string{*ETCDServer}))
	if err != nil {
		log.Println("ERROR: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	t, err := template.New("index.html").ParseFiles("templates/index.html", "templates/base.html")
	if err != nil {
		log.Println("ERROR: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	u, _ := getUser(r)
	c.Projects = cleanProjects(c.Projects, r, u)
	sort.Sort(ByName(c.Projects))
	t.ExecuteTemplate(w, "base", map[string]interface{}{"Projects": c.Projects, "User": u, "Page": "home"})
}

var validPathWithEnv = regexp.MustCompile("^/(deployLog|commits)/(.*)$")

func extractDeployLogHandler(fn func(http.ResponseWriter, *http.Request, string, goship.Environment)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPathWithEnv.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		c, err := goship.ParseETCD(etcd.NewClient([]string{*ETCDServer}))
		if err != nil {
			log.Println("ERROR: ", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// auth check for user
		u, _ := getUser(r)
		c.Projects = cleanProjects(c.Projects, r, u)
		// get project name and env from url
		a := strings.Split(m[2], "-")
		l := len(a)
		environmentName := a[l-1]
		var projectName string
		if m[1] == "commits" {
			projectName = m[2]
		} else {
			projectName = strings.Join(a[0:l-1], "-")
		}
		e, err := goship.EnvironmentFromName(c.Projects, projectName, environmentName)
		if err != nil {
			log.Println("ERROR: Can't get environment from name", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fn(w, r, m[2], *e)
	}
}

func extractCommitHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPathWithEnv.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

var validPathWithEnvAndTime = regexp.MustCompile("^/(output)/(.*)/(.*)$")

func extractOutputHandler(fn func(http.ResponseWriter, *http.Request, string, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPathWithEnvAndTime.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2], m[3])
	}
}

func isAuthCollaborator(owner, repo, user string) (bool, error) {
	gt := os.Getenv(gitHubAPITokenEnvVar)
	t := &oauth.Transport{
		Token: &oauth.Token{AccessToken: gt},
	}
	github.NewClient(t.Client())
	c := github.NewClient(t.Client())
	m := false
	m, _, err := c.Repositories.IsCollaborator(owner, repo, user)
	if err != nil {
		log.Print("Failure getting Collaboration Status of User %s %s %s", owner, user, repo)
	}
	return m, err
}

type User struct {
	UserName, UserAvatar string
	auth                 auth
}

func getUser(r *http.Request) (User, error) {
	u := User{}
	session, err := store.Get(r, session_name)
	if err != nil {
		return u, errors.New("Error Getting User Session")
	}
	if _, ok := session.Values["userName"]; !ok {
		u.UserName = "genericUSER"
		u.UserAvatar = "https://camo.githubusercontent.com/33a7d9a138ac73ece82dee977c216eb13dffc984/687474703a2f2f692e696d6775722e636f6d2f524c766b486b612e706e67"
		return u, errors.New("No username")
	}
	if _, ok := session.Values["avatarURL"]; !ok {
		return u, errors.New("No avatar")
	}
	// Check return of nil
	u.UserName = session.Values["userName"].(string)
	u.UserAvatar = session.Values["avatarURL"].(string)
	return u, nil
}

func checkAuth(fn http.HandlerFunc, a auth) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := getUser(r)
		if err != nil && a.authorization != false {
			log.Printf("error getting a logged in user %s", err)
			http.Redirect(w, r, os.Getenv("GITHUB_CALLBACK_URL")+"/auth/github/login", 301)

			return
		}
		fn.ServeHTTP(w, r)
	})
}

type cleanProject []goship.Project

// remove projects where user has no access
func cleanProjects(n cleanProject, r *http.Request, u User) cleanProject {
	f_projects := []goship.Project{}
	for _, p := range n {
		a := isCollaborator(p.RepoOwner, p.RepoName, u.UserName)
		// If user does not have access to the project remove it.
		if a == true || authentication.authorization == false {
			f_projects = append(f_projects, p)
		}
	}
	return f_projects
}

func loginHandler(providerName string, auth bool) http.HandlerFunc {
	if auth == true {
		provider, err := gomniauth.Provider(providerName)
		if err != nil {
			panic(err)
		}
		return func(w http.ResponseWriter, r *http.Request) {

			state := gomniauth.NewState("after", "success")

			authUrl, err := provider.GetBeginAuthURL(state, nil)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// redirect
			http.Redirect(w, r, authUrl, http.StatusFound)
		}
	} else {
		return func(w http.ResponseWriter, r *http.Request) {}
	}
}

func callbackHandler(providerName string, auth bool) http.HandlerFunc {
	// Handle error
	if auth == true {
		provider, err := gomniauth.Provider(providerName)
		if err != nil {
			panic(err)
		}
		return func(w http.ResponseWriter, r *http.Request) {

			omap, err := objx.FromURLQuery(r.URL.RawQuery)
			if err != nil {
				log.Printf("error getting resp from callback")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			creds, err := provider.CompleteAuth(omap)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			user, userErr := provider.GetUser(creds)
			if userErr != nil {
				log.Printf("Failed to get user from Github %s", user)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			session, err := store.Get(r, session_name)
			if err != nil {
				log.Fatal("Can't get a session store")
			}

			session.Options = &sessions.Options{
				Path:     "/",
				MaxAge:   86400 * 7,
				HttpOnly: true,
			}

			session.Values["userName"] = user.Nickname()
			session.Values["avatarURL"] = user.AvatarURL()
			log.Print("saving session")
			session.Save(r, w)

			http.Redirect(w, r, os.Getenv("GITHUB_CALLBACK_URL"), http.StatusFound)

		}
	} else {
		return func(w http.ResponseWriter, r *http.Request) {}
	}
}

type auth struct {
	authorization       bool
	githubRandomHashKey string
	githubOmniauthID    string
	githubOmniauthKey   string
	githubCallbackURL   string
}

func getAuth() auth {
	a := auth{}
	a.githubRandomHashKey = os.Getenv("GITHUB_RANDOM_HASH_KEY")
	a.githubOmniauthID = os.Getenv("GITHUB_OMNI_AUTH_ID")
	a.githubOmniauthKey = os.Getenv("GITHUB_OMNI_AUTH_KEY")
	a.githubCallbackURL = os.Getenv("GITHUB_CALLBACK_URL")
	a.authorization = true
	// Let user know if a key is missing and that auth is disabled.
	if a.githubRandomHashKey == "" || a.githubOmniauthID == "" || a.githubOmniauthKey == "" || a.githubCallbackURL == "" {
		log.Printf("Missing one or more Gomniauth Environment Variables: Running with with limited functionality! \n githubRandomHashKey [%s] \n githubOmniauthID [%s] \n githubOmniauthKey[%s] \n githubCallbackURL[%s]",
			a.githubRandomHashKey,
			a.githubOmniauthID,
			a.githubOmniauthKey,
			a.githubCallbackURL)
		a.authorization = false
	} else {
		gomniauth.SetSecurityKey(a.githubRandomHashKey)
		gomniauth.WithProviders(
			githubOauth.New(a.githubOmniauthID, a.githubOmniauthKey, a.githubCallbackURL+"/auth/github/callback"),
		)
		a.authorization = true
	}
	return a
}

func isCollaborator(owner, repo, user string) bool {
	if authentication.authorization == true {

		gt := os.Getenv(gitHubAPITokenEnvVar)
		t := &oauth.Transport{
			Token: &oauth.Token{AccessToken: gt},
		}
		github.NewClient(t.Client())
		c := github.NewClient(t.Client())
		m, _, err := c.Repositories.IsCollaborator(owner, repo, user)
		if err != nil {
			log.Print("Failure getting Collaboration Status of User %s %s %s", owner, user, repo)
			return false
		}
		return m
	} else {
		return true
	}
}

func main() {
	authentication = getAuth()
	log.Print("Authorization is On? %s", authentication.authorization)

	log.Printf("Starting Goship...")
	if err := os.Mkdir(*dataPath, 0777); err != nil && !os.IsExist(err) {
		log.Fatal("could not create data dir: ", err)
	}
	flag.Parse()
	go h.run()
	http.HandleFunc("/", checkAuth(HomeHandler, authentication))
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, r.URL.Path[1:])
	})
	http.Handle("/web_push", websocket.Handler(websocketHandler))
	http.HandleFunc("/deploy", checkAuth(DeployPage, authentication))
	http.HandleFunc("/deployLog/", checkAuth(extractDeployLogHandler(DeployLogHandler), authentication))
	http.HandleFunc("/output/", checkAuth(extractOutputHandler(DeployOutputHandler), authentication))
	http.HandleFunc("/commits/", checkAuth(extractCommitHandler(ProjCommitsHandler), authentication))
	http.HandleFunc("/deploy_handler", checkAuth(DeployHandler, authentication))
	http.HandleFunc("/auth/github/login", loginHandler("github", authentication.authorization))
	http.HandleFunc("/auth/github/callback", callbackHandler("github", authentication.authorization))
	fmt.Printf("Running on %s\n", *bindAddress)
	log.Fatal(http.ListenAndServe(*bindAddress, nil))
}
