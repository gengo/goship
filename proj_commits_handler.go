package main

import (
	"encoding/json"
	"net/http"

	"github.com/coreos/go-etcd/etcd"
	goship "github.com/gengo/goship/lib"
	"github.com/gengo/goship/lib/acl"
	"github.com/gengo/goship/lib/auth"
	githublib "github.com/gengo/goship/lib/github"
	"github.com/golang/glog"
)

type ProjCommitsHandler struct {
	ac  acl.AccessControl
	gcl githublib.Client
	ecl *etcd.Client
}

func (h ProjCommitsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, projName string) {
	c, err := goship.ParseETCD(h.ecl)
	if err != nil {
		glog.Errorf("Parsing etc: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	u, err := auth.CurrentUser(r)
	if err != nil {
		glog.Errorf("Failed to get current user: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	proj, err := goship.ProjectFromName(c.Projects, projName)
	if err != nil {
		glog.Errorf("Failed to get project from name: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Remove projects that the user is not a collaborator on...
	fp := acl.ReadableProjects(h.ac, []goship.Project{*proj}, u)
	p, err := retrieveCommits(h.gcl, h.ac, r, fp[0], c.DeployUser)
	if err != nil {
		glog.Errorf("Failed to retrieve commits: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	j, err := json.Marshal(p)
	if err != nil {
		glog.Errorf("Failed to marshal response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(j)
	if err != nil {
		glog.Errorf("Failed to send response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
