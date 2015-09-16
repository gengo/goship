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
	"github.com/gengo/goship/lib/auth"
	"github.com/gengo/goship/lib/config"
	"github.com/gengo/goship/lib/notification"
	"github.com/gengo/goship/lib/revision"
	"github.com/golang/glog"
	"golang.org/x/net/context"
)

type DeployHandler struct {
	ecl  *etcd.Client
	ctrl revision.Control
	hub  *notification.Hub
}

func (h DeployHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	c, err := config.Load(h.ecl)
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

	var (
		user              = u.Name
		projName, envName string
		deploy            RevRange
		src               = RevRange{
			From: revision.Revision(r.FormValue("from_source_revision")),
			To:   revision.Revision(r.FormValue("to_source_revision")),
		}
	)
	for _, spec := range []struct {
		name  string
		value *string
	}{
		{name: "project", value: &projName},
		{name: "environment", value: &envName},
		{name: "from_revision", value: (*string)(&deploy.From)},
		{name: "to_revision", value: (*string)(&deploy.To)},
	} {
		*spec.value = r.FormValue(spec.name)
		if *spec.value == "" {
			glog.Errorf("%s not specified", spec.name)
			http.Error(w, fmt.Sprintf("%s not specified", spec.name), http.StatusBadRequest)
			return
		}
	}
	proj, err := config.ProjectFromName(c.Projects, projName)
	if err != nil {
		http.Error(w, "no such project", http.StatusNotFound)
		return
	}
	env, err := config.EnvironmentFromName(c.Projects, projName, envName)
	if err != nil {
		http.Error(w, "no such project/environment", http.StatusNotFound)
		return
	}

	h.deploy(ctx, w, c, user, proj, *env, deploy, src)
}

func (h DeployHandler) deploy(ctx context.Context, w http.ResponseWriter, c config.Config, user string, proj config.Project, env config.Environment, deploy, src RevRange) {
	if c.Notify != "" {
		err := startNotify(c.Notify, user, proj.Name, env.Name)
		if err != nil {
			glog.Errorf("Failed to notify start-deployment event of %s (%s): %v", proj.Name, env.Name, err)
		}
	}

	deployTime := time.Now()
	success := true
	command := deployCommand(env)
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
	repo := proj.SourceRepo()
	glog.Infof("Starting deployment of %s-%s (%s/%s) from %s to %s; requested by %s", proj.Name, env.Name, repo.RepoOwner, repo.RepoName, deploy.From, deploy.To, user)
	if err = cmd.Start(); err != nil {
		glog.Errorf("Could not run deployment command: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go h.sendOutput(&wg, bufio.NewScanner(stdout), proj.Name, env.Name, deployTime)
	go h.sendOutput(&wg, bufio.NewScanner(stderr), proj.Name, env.Name, deployTime)
	wg.Wait()

	err = cmd.Wait()
	if err != nil {
		success = false
		glog.Errorf("Deployment of %s failed: %v", proj.Name, err)
	} else {
		glog.Infof("Successfully deployed %s", proj.Name)
	}
	if c.Notify != "" {
		err = endNotify(c.Notify, proj.Name, env.Name, success)
		if err != nil {
			glog.Errorf("Failed to notify start-deployment event of %s (%s): %v", proj.Name, env.Name, err)
		}
	}

	if (c.Pivotal.Token != "") && success {
		err := config.PostToPivotal(c.Pivotal, env.Name, repo.RepoOwner, repo.RepoName, string(deploy.From), string(deploy.To))
		if err != nil {
			glog.Errorf("Failed to post to pivotal: %v", err)
		} else {
			glog.Infof("Pivotal Token: %s %s", c.Pivotal.Token)
		}
	}

	err = h.insertEntry(ctx, proj, env, deploy, src, user, success, deployTime)
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

// deployCommand returns the deployment command for a given
// environment as a string slice that has been split on spaces.
func deployCommand(e config.Environment) []string {
	// TODO(yugui) better handling of shell escape
	return strings.Split(e.Deploy, " ")
}

func (h DeployHandler) insertEntry(ctx context.Context, proj config.Project, env config.Environment, deploy, src RevRange, user string, success bool, time time.Time) error {
	basename := fmt.Sprintf("%s-%s", proj.Name, env.Name)
	path := path.Join(*dataPath, basename+".json")
	err := prepareDataFiles(path)
	if err != nil {
		return err
	}

	e, err := readEntries(basename)
	if err != nil {
		return err
	}

	repo := proj.SourceRepo()
	var msg string
	if src.To != "" {
		msg, err = h.ctrl.SourceRevMessage(ctx, proj, src.To)
		if err != nil {
			glog.Errorf("Failed to get commit %s (%s/%s): %v", src.To, repo.RepoOwner, repo.RepoName, err)
			msg = ""
		}
	}
	var diffURL string
	if src.From != "" && src.To != "" {
		diffURL = h.ctrl.SourceDiffURL(proj, src.From, src.To)
	}
	d := DeployLogEntry{
		Range:         deploy,
		DiffURL:       diffURL,
		ToRevisionMsg: msg,
		User:          user,
		Time:          time,
		Success:       success,
	}
	e = append(e, d)
	err = writeJSON(e, path)
	if err != nil {
		return err
	}
	return nil
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
