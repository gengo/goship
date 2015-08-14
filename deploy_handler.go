package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-etcd/etcd"
	goship "github.com/gengo/goship/lib"
	"github.com/gengo/goship/lib/auth"
	"github.com/gengo/goship/lib/notification"
	"github.com/golang/glog"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type DeployHandler struct {
	ecl *etcd.Client
	hub *notification.Hub
}

func (h DeployHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := goship.ParseETCD(h.ecl)
	if err != nil {
		glog.Errorf("Failed to fetch latest configuration: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	u, err := auth.CurrentUser(r)
	if err != nil {
		glog.Errorf("Failed to fetch current user: %v", err)
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
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
			glog.Errorf("Failed to notify start-deployment event of %s (%s): %v", p, env, err)
		}
	}

	deployTime := time.Now()
	success := true
	command, err := getDeployCommand(c.Projects, p, env)
	if err != nil {
		glog.Errorf("Failed to fetch deploy command: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cmd := exec.Command(command[0], command[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		glog.Errorf("Could not get stdout of command: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		glog.Errorf("Could not get stderr of command: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	glog.Infof("Starting deployment of %s-%s (%s/%s) from %s to %s; requested by %s", p, env, owner, name, fromRevision, toRevision, user)
	if err = cmd.Start(); err != nil {
		glog.Errorf("Could not run deployment command: %v", err)
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
		glog.Errorf("Deployment of %s failed: %v", p, err)
	} else {
		glog.Infof("Successfully deployed %s", p)
	}
	if c.Notify != "" {
		err = endNotify(c.Notify, p, env, success)
		if err != nil {
			glog.Errorf("Failed to notify start-deployment event of %s (%s): %v", p, env, err)
		}
	}

	if (c.Pivotal.Token != "") && (c.Pivotal.Project != "") && success {
		err := goship.PostToPivotal(c.Pivotal, env, owner, name, toRevision, fromRevision)
		if err != nil {
			glog.Errorf("Failed to post to pivotal: %v", err)
		} else {
			glog.Infof("Pivotal Info: %s %s", c.Pivotal.Token, c.Pivotal.Project)
		}
	}

	err = insertEntry(fmt.Sprintf("%s-%s", p, env), owner, name, fromRevision, toRevision, user, success, deployTime)
	if err != nil {
		glog.Errorf("Failed to insert an entry: %v", err)
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
			glog.Errorf("Failed to marshal output into JSON: %v", err)
		}
		h.hub.Broadcast(string(cmdOutput))

		go appendDeployOutput(fmt.Sprintf("%s-%s", p, e), t, deployTime)
	}
	if err := scanner.Err(); err != nil {
		glog.Errorf("Failed to scan deploy output: %v", err)
		return
	}
}

func stripANSICodes(t string) string {
	ansi := regexp.MustCompile(`\x1B\[[0-9;]{1,4}[mK]`)
	return ansi.ReplaceAllString(t, "")
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

func startNotify(n, user, p, env string) error {
	msg := fmt.Sprintf("%s is deploying %s to *%s*.", user, p, env)
	err := notify(n, msg)
	if err != nil {
		return err
	}
	return nil
}

func notify(n, msg string) error {
	cmd := exec.Command(n, msg)
	err := cmd.Run()
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

// getDeployCommand returns the deployment command for a given
// environment as a string slice that has been split on spaces.
func getDeployCommand(projects []goship.Project, projectName, environmentName string) (s []string, err error) {
	e, err := goship.EnvironmentFromName(projects, projectName, environmentName)
	if err != nil {
		return s, err
	}

	return strings.Split(e.Deploy, " "), nil
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
		glog.Errorf("Failed to get commit %s (%s/%s): %v", toRevision, owner, repoName, err)
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

func diffURL(owner, repoName, fromRevision, toRevision string) string {
	return fmt.Sprintf("https://github.com/%s/%s/compare/%s...%s", owner, repoName, fromRevision, toRevision)
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

func writeJSON(d []DeployLogEntry, file string) error {
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(file, b, 0755)
}
