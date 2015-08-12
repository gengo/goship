package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-etcd/etcd"
	goship "github.com/gengo/goship/lib"
	"github.com/gengo/goship/lib/auth"
	"github.com/gengo/goship/lib/notification"
	helpers "github.com/gengo/goship/lib/view-helpers"
	_ "github.com/gengo/goship/plugins"
	"github.com/gengo/goship/plugins/plugin"
	"github.com/google/go-github/github"
	"golang.org/x/net/context"
	"golang.org/x/net/websocket"
	"golang.org/x/oauth2"
)

var (
	bindAddress       = flag.String("b", "localhost:8000", "Address to bind (default localhost:8000)")
	sshPort           = "22"
	keyPath           = flag.String("k", "id_rsa", "Path to private SSH key (default id_rsa)")
	dataPath          = flag.String("d", "data/", "Path to data directory (default ./data/)")
	staticFilePath    = flag.String("s", "static/", "Path to directory for static files (default ./static/)")
	ETCDServer        = flag.String("e", "http://127.0.0.1:4001", "Etcd Server (default http://127.0.0.1:4001)")
	cookieSessionHash = flag.String("c", "COOKIE-SESSION-HASH", "Random cookie session key (default jhjhjhjhjhjjhjhhj)")
	defaultUser       = flag.String("u", "genericUser", "Default User if non auth (default genericUser)")
	defaultAvatar     = flag.String("a", "https://camo.githubusercontent.com/33a7d9a138ac73ece82dee977c216eb13dffc984/687474703a2f2f692e696d6775722e636f6d2f524c766b486b612e706e67", "Default Avatar (default goship gopher image)")
	confirmDeployFlag = flag.Bool("f", true, "Flag to always ask for confirmation before deploying")
)

func diffURL(owner, repoName, fromRevision, toRevision string) string {
	return fmt.Sprintf("https://github.com/%s/%s/compare/%s...%s", owner, repoName, fromRevision, toRevision)
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
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: gt})
	c := github.NewClient(oauth2.NewClient(oauth2.NoContext, ts))
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

// DeployLogHandler shows data about the environment including the deploy log.
func DeployLogHandler(w http.ResponseWriter, r *http.Request, fullEnv string, environment goship.Environment, projectName string) {
	u, err := auth.CurrentUser(r)
	if err != nil {
		log.Println("Failed to get User! ")
		http.Error(w, err.Error(), http.StatusUnauthorized)
	}
	d, err := readEntries(fullEnv)
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
	js, css := getAssetsTemplates()
	t.ExecuteTemplate(w, "base", map[string]interface{}{"Javascript": js, "Stylesheet": css, "Deployments": d, "User": u, "Env": fullEnv, "Environment": environment, "ProjectName": projectName})
}

func ProjCommitsHandler(w http.ResponseWriter, r *http.Request, projName string) {
	c, err := goship.ParseETCD(etcd.NewClient([]string{*ETCDServer}))
	if err != nil {
		log.Println("ERROR: Parsing etc ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	u, err := auth.CurrentUser(r)
	if err != nil {
		log.Println("ERROR:  Getting User", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	proj, err := goship.ProjectFromName(c.Projects, projName)
	if err != nil {
		log.Println("ERROR:  Getting Project from name", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Remove projects that the user is not a collaborator on...
	fp := removeUnauthorizedProjects([]goship.Project{*proj}, r, u)
	p, err := retrieveCommits(r, fp[0], c.DeployUser)
	if err != nil {
		log.Println("ERROR: Retrieving Commits ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	j, err := json.Marshal(p)
	if err != nil {
		log.Println("ERROR: Marshalling Retrieving Commits ", err)
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

func (h DeployHandler) sendOutput(wg *sync.WaitGroup, scanner *bufio.Scanner, p, e string, deployTime time.Time) {
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
		h.hub.Broadcast(string(cmdOutput))

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
	msg := fmt.Sprintf("%s is deploying %s to *%s*.", user, p, env)
	err := notify(n, msg)
	if err != nil {
		return err
	}
	return nil
}

func endNotify(n, p, env string, success bool) error {
	msg := fmt.Sprintf("%s successfully deployed to *%s*.", p, env)
	if !success {
		msg = fmt.Sprintf("%s deployment to *%s* failed.", p, env)
	}
	err := notify(n, msg)
	if err != nil {
		return err
	}
	return nil
}

type DeployHandler struct {
	ecl *etcd.Client
	hub *notification.Hub
}

func (h DeployHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := goship.ParseETCD(h.ecl)
	if err != nil {
		log.Println("ERROR: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	u, err := auth.CurrentUser(r)
	if err != nil {
		log.Println("Failed to get a User! ")
		http.Error(w, err.Error(), http.StatusUnauthorized)
	}
	user := u.Name
	p := r.FormValue("project")
	env := r.FormValue("environment")
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
	go h.sendOutput(&wg, bufio.NewScanner(stdout), p, env, deployTime)
	go h.sendOutput(&wg, bufio.NewScanner(stderr), p, env, deployTime)
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
		err := goship.PostToPivotal(c.Pivotal, env, owner, name, toRevision, fromRevision)
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

// CommentHandler allows you to update a comment on an environment
// i.e. http://127.0.0.1:8000/comment?environment=staging&project=admin&comment=DONOTDEPLOYPLEASE!
func CommentHandler(w http.ResponseWriter, r *http.Request) {
	c := etcd.NewClient([]string{*ETCDServer})
	p := r.FormValue("project")
	env := r.FormValue("environment")
	comment := r.FormValue("comment")
	err := goship.SetComment(c, p, env, comment)
	if err != nil {
		log.Println("ERROR: ", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// LockHandler allows you to lock an environment
// http://127.0.0.1:8000/lock?environment=staging&project=admin
func LockHandler(w http.ResponseWriter, r *http.Request) {
	c := etcd.NewClient([]string{*ETCDServer})
	p := r.FormValue("project")
	env := r.FormValue("environment")
	err := goship.LockEnvironment(c, p, env, "true")
	if err != nil {
		log.Println("ERROR: ", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// UnLockHandler allows you to unlock an environment
// http://127.0.0.1:8000/unlock?environment=staging&project=admin
func UnLockHandler(w http.ResponseWriter, r *http.Request) {
	c := etcd.NewClient([]string{*ETCDServer})
	p := r.FormValue("project")
	env := r.FormValue("environment")
	err := goship.LockEnvironment(c, p, env, "false")
	if err != nil {
		log.Println("ERROR: ", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func DeployPage(w http.ResponseWriter, r *http.Request) {
	user, err := auth.CurrentUser(r)
	if err != nil {
		log.Println("Failed to Get User")
		http.Error(w, err.Error(), http.StatusUnauthorized)
	}
	p := r.FormValue("project")
	env := r.FormValue("environment")
	fromRevision := r.FormValue("from_revision")
	toRevision := r.FormValue("to_revision")
	repoOwner := r.FormValue("repo_owner")
	repoName := r.FormValue("repo_name")
	t, err := template.New("deploy.html").ParseFiles("templates/deploy.html", "templates/base.html")
	if err != nil {
		log.Println("ERROR: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	js, css := getAssetsTemplates()
	t.ExecuteTemplate(w, "base", map[string]interface{}{"Javascript": js, "Stylesheet": css, "Project": p, "Env": env, "User": user, "BindAddress": bindAddress, "RepoOwner": repoOwner, "RepoName": repoName, "ToRevision": toRevision, "FromRevision": fromRevision})
}

// ByName is the interface for sorting projects
type ByName []goship.Project

func (slice ByName) Len() int           { return len(slice) }
func (slice ByName) Less(i, j int) bool { return slice[i].Name < slice[j].Name }
func (slice ByName) Swap(i, j int)      { slice[i], slice[j] = slice[j], slice[i] }

// HomeHandler is the main home screen
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	c, err := goship.ParseETCD(etcd.NewClient([]string{*ETCDServer}))
	if err != nil {
		log.Printf("Failed to Parse to ETCD data %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	u, err := auth.CurrentUser(r)
	if err != nil {
		log.Println("Failed to get User! ")
		http.Error(w, err.Error(), http.StatusUnauthorized)
	}
	t, err := template.New("index.html").ParseFiles("templates/index.html", "templates/base.html")
	if err != nil {
		log.Printf("Failed to parse template: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	c.Projects = removeUnauthorizedProjects(c.Projects, r, u)

	sort.Sort(ByName(c.Projects))

	// apply each plugin
	for _, pl := range plugin.Plugins {
		err := pl.Apply(c)
		if err != nil {
			log.Printf("Failed to apply plugin: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
	js, css := getAssetsTemplates()
	gt := os.Getenv(gitHubAPITokenEnvVar)
	pt := c.Pivotal.Token

	t.ExecuteTemplate(w, "base", map[string]interface{}{"Javascript": js, "Stylesheet": css, "Projects": c.Projects, "User": u, "Page": "home", "ConfirmDeployFlag": *confirmDeployFlag, "GithubToken": gt, "PivotalToken": pt})
}

func getAssetsTemplates() (js, css template.HTML) {
	sfp, err := filepath.Abs(*staticFilePath)
	if err != nil {
		var tmpl = template.HTML("")
		log.Printf("Failed to locate static file path: %s", err)
		return tmpl, tmpl
	}
	js = helpers.MakeJavascriptTemplate(path.Join(sfp, "js"))
	css = helpers.MakeStylesheetTemplate(path.Join(sfp, "css"))
	return js, css
}

var validPathWithEnv = regexp.MustCompile("^/(deployLog|commits)/(.*)$")

func extractDeployLogHandler(fn func(http.ResponseWriter, *http.Request, string, goship.Environment, string)) http.HandlerFunc {
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
		u, err := auth.CurrentUser(r)
		if err != nil {
			log.Println("Failed to get a user while deploying in Auth Mode! ")
			http.Error(w, err.Error(), http.StatusUnauthorized)
		}
		c.Projects = removeUnauthorizedProjects(c.Projects, r, u)
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
		fn(w, r, m[2], *e, projectName)
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

// remove projects where user is not a collaborator
func removeUnauthorizedProjects(cp []goship.Project, r *http.Request, u auth.User) (fp []goship.Project) {
	if !auth.Enabled() {
		return cp
	}

	for _, p := range cp {
		a := isCollaborator(p.RepoOwner, p.RepoName, u.Name)
		if a == true {
			fp = append(fp, p)
		}
	}
	return fp
}

//  set projects to lock where user is only in a pull only repo and append a comment
func filterProject(p goship.Project, r *http.Request, u auth.User) goship.Project {
	if !auth.Enabled() {
		return p
	}

	g := newGithubClient()
	for i, e := range p.Environments {
		// If the repo isn't already locked.. lock it if the user doesnt have permission
		// and add to the comments
		if e.IsLocked {
			if p.Environments[i].Comment != "" {
				p.Environments[i].Comment = p.Environments[i].Comment + " | "
			}
			p.Environments[i].Comment = p.Environments[i].Comment + "repo is locked."
			continue
		}

		lock, err := userHasDeployPermission(g, p.RepoOwner, p.RepoName, u.Name)
		if err != nil {
			log.Printf("Error getting Lock Permission: Locking anyway for safety %s", err)
		}
		p.Environments[i].IsLocked = !lock
		// Add a line break if there is already a comment
		if p.Environments[i].Comment != "" {
			p.Environments[i].Comment = p.Environments[i].Comment + " | "
		}
		if !lock {
			p.Environments[i].Comment = p.Environments[i].Comment + "you do not have permission to deploy "
		}
	}
	return p
}

func main() {
	flag.Parse()
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	auth.Initialize(auth.User{Name: *defaultUser, Avatar: *defaultAvatar}, []byte(*cookieSessionHash))

	log.Printf("Starting Goship...")
	if err := os.Mkdir(*dataPath, 0777); err != nil && !os.IsExist(err) {
		log.Fatal("could not create data dir: ", err)
	}
	hub := notification.NewHub(ctx)
	ecl := etcd.NewClient([]string{*ETCDServer})

	http.Handle("/", auth.AuthenticateFunc(HomeHandler))
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, r.URL.Path[1:])
	})
	http.Handle("/web_push", websocket.Handler(hub.AcceptConnection))
	http.Handle("/deploy", auth.AuthenticateFunc(DeployPage))
	http.Handle("/deployLog/", auth.AuthenticateFunc(extractDeployLogHandler(DeployLogHandler)))
	http.Handle("/output/", auth.AuthenticateFunc(extractOutputHandler(DeployOutputHandler)))
	http.Handle("/commits/", auth.AuthenticateFunc(extractCommitHandler(ProjCommitsHandler)))
	http.Handle("/deploy_handler", auth.Authenticate(DeployHandler{ecl: ecl, hub: hub}))
	http.Handle("/lock", auth.AuthenticateFunc(LockHandler))
	http.Handle("/unlock", auth.AuthenticateFunc(UnLockHandler))
	http.Handle("/comment", auth.AuthenticateFunc(CommentHandler))
	http.HandleFunc("/auth/github/login", auth.LoginHandler)
	http.HandleFunc("/auth/github/callback", auth.CallbackHandler)
	fmt.Printf("Running on %s\n", *bindAddress)
	log.Fatal(http.ListenAndServe(*bindAddress, nil))
}
