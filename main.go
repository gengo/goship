package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/coreos/go-etcd/etcd"
	"github.com/gengo/goship/handlers/comment"
	deploypage "github.com/gengo/goship/handlers/deploy-page"
	"github.com/gengo/goship/handlers/lock"
	goship "github.com/gengo/goship/lib"
	"github.com/gengo/goship/lib/acl"
	"github.com/gengo/goship/lib/auth"
	githublib "github.com/gengo/goship/lib/github"
	"github.com/gengo/goship/lib/notification"
	helpers "github.com/gengo/goship/lib/view-helpers"
	_ "github.com/gengo/goship/plugins"
	"github.com/golang/glog"
	ghandlers "github.com/gorilla/handlers"
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
	requestLog        = flag.String("request-log", "-", "destination of request log. '-' means stdout")
)

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
			glog.Errorf("Failed to get current configuration: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// auth check for user
		u, err := auth.CurrentUser(r)
		if err != nil {
			glog.Error("Failed to get a user while deploying in Auth Mode: %v", err)
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
			glog.Errorf("Can't get environment from name: %v", err)
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

func buildHandler(ctx context.Context) (http.Handler, error) {
	gcl, err := newGithubClient()
	if err != nil {
		glog.Errorf("Failed to build github client: %v", err)
		return nil, err
	}

	ac := acl.Null
	if auth.Enabled() {
		ac = acl.NewGithub(gcl)
	}

	if err := os.Mkdir(*dataPath, 0777); err != nil && !os.IsExist(err) {
		glog.Errorf("could not create data dir: %v", err)
		return nil, err
	}

	hub := notification.NewHub(ctx)
	ecl := etcd.NewClient([]string{*ETCDServer})
	assets := helpers.New(*staticFilePath)

	mux := http.NewServeMux()
	mux.Handle("/", auth.Authenticate(HomeHandler{ac: ac, ecl: ecl, assets: assets}))
	mux.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, r.URL.Path[1:])
	})

	dph, err := deploypage.New(assets, fmt.Sprintf("ws://%s/web_push", *bindAddress))
	if err != nil {
		glog.Errorf("Failed to build deploy page handler: %v", err)
		return nil, err
	}
	mux.Handle("/deploy", auth.Authenticate(dph))
	mux.Handle("/web_push", websocket.Handler(hub.AcceptConnection))

	dlh := DeployLogHandler{assets: assets}
	mux.Handle("/deployLog/", auth.AuthenticateFunc(extractDeployLogHandler(ac, ecl, dlh.ServeHTTP)))
	mux.Handle("/output/", auth.AuthenticateFunc(extractOutputHandler(DeployOutputHandler)))

	pch := ProjCommitsHandler{ac: ac, gcl: gcl, ecl: ecl}
	mux.Handle("/commits/", auth.AuthenticateFunc(extractCommitHandler(pch.ServeHTTP)))
	mux.Handle("/deploy_handler", auth.Authenticate(DeployHandler{ecl: ecl, hub: hub}))
	mux.Handle("/lock", auth.Authenticate(lock.NewLock(ecl)))
	mux.Handle("/unlock", auth.Authenticate(lock.NewUnlock(ecl)))
	mux.Handle("/comment", auth.Authenticate(comment.New(ecl)))
	mux.HandleFunc("/auth/github/login", auth.LoginHandler)
	mux.HandleFunc("/auth/github/callback", auth.CallbackHandler)

	return mux, nil
}

func main() {
	flag.Parse()
	glog.Infof("Starting Goship...")

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	auth.Initialize(auth.User{Name: *defaultUser, Avatar: *defaultAvatar}, []byte(*cookieSessionHash))

	h, err := buildHandler(ctx)
	if err != nil {
		glog.Fatal(err)
	}
	w := io.WriteCloser(os.Stdout)
	if *requestLog != "-" {
		w, err = os.OpenFile(*requestLog, os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			glog.Fatalf("Cannot open request log %s: %v", *requestLog, err)
		}
		defer w.Close()
	}
	h = ghandlers.CombinedLoggingHandler(w, h)

	fmt.Printf("Running on %s\n", *bindAddress)
	s := &http.Server{
		Addr:    *bindAddress,
		Handler: h,
	}
	if err := s.ListenAndServe(); err != nil {
		glog.Fatal(err)
	}
}
