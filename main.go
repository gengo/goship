package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/coreos/go-etcd/etcd"
	goship "github.com/gengo/goship/lib"
	"github.com/gengo/goship/lib/acl"
	"github.com/gengo/goship/lib/auth"
	githublib "github.com/gengo/goship/lib/github"
	"github.com/gengo/goship/lib/notification"
	helpers "github.com/gengo/goship/lib/view-helpers"
	_ "github.com/gengo/goship/plugins"
	"golang.org/x/net/context"
	"golang.org/x/net/websocket"
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

func extractDeployLogHandler(ac acl.AccessControl, ecl *etcd.Client, fn func(http.ResponseWriter, *http.Request, string, goship.Environment, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPathWithEnv.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		c, err := goship.ParseETCD(ecl)
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
		c.Projects = acl.ReadableProjects(ac, c.Projects, u)
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

const (
	gitHubAPITokenEnvVar = "GITHUB_API_TOKEN"
)

func newGithubClient() (githublib.Client, error) {
	gt := os.Getenv(gitHubAPITokenEnvVar)
	if gt == "" {
		return nil, fmt.Errorf("environment variable %s not defined", gitHubAPITokenEnvVar)
	}
	return githublib.NewClient(gt), nil
}

func main() {
	flag.Parse()
	log.Printf("Starting Goship...")

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	auth.Initialize(auth.User{Name: *defaultUser, Avatar: *defaultAvatar}, []byte(*cookieSessionHash))

	gcl, err := newGithubClient()
	if err != nil {
		log.Panicf("Failed to build github client: %v", err)
	}

	ac := acl.Null
	if auth.Enabled() {
		ac = acl.NewGithub(gcl)
	}

	if err := os.Mkdir(*dataPath, 0777); err != nil && !os.IsExist(err) {
		log.Fatal("could not create data dir: ", err)
	}

	hub := notification.NewHub(ctx)
	ecl := etcd.NewClient([]string{*ETCDServer})

	http.Handle("/", auth.Authenticate(HomeHandler{ac: ac, ecl: ecl}))
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, r.URL.Path[1:])
	})
	http.Handle("/web_push", websocket.Handler(hub.AcceptConnection))
	http.Handle("/deploy", auth.AuthenticateFunc(DeployPage))
	http.Handle("/deployLog/", auth.AuthenticateFunc(extractDeployLogHandler(ac, ecl, DeployLogHandler)))
	http.Handle("/output/", auth.AuthenticateFunc(extractOutputHandler(DeployOutputHandler)))

	pch := ProjCommitsHandler{ac: ac, gcl: gcl, ecl: ecl}
	http.Handle("/commits/", auth.AuthenticateFunc(extractCommitHandler(pch.ServeHTTP)))
	http.Handle("/deploy_handler", auth.Authenticate(DeployHandler{ecl: ecl, hub: hub}))
	http.Handle("/lock", auth.AuthenticateFunc(LockHandler))
	http.Handle("/unlock", auth.AuthenticateFunc(UnLockHandler))
	http.Handle("/comment", auth.AuthenticateFunc(CommentHandler))
	http.HandleFunc("/auth/github/login", auth.LoginHandler)
	http.HandleFunc("/auth/github/callback", auth.CallbackHandler)
	fmt.Printf("Running on %s\n", *bindAddress)
	log.Fatal(http.ListenAndServe(*bindAddress, nil))
}
