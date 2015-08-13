package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/coreos/go-etcd/etcd"
	goship "github.com/gengo/goship/lib"
	"github.com/gengo/goship/lib/acl"
	"github.com/gengo/goship/lib/auth"
	githublib "github.com/gengo/goship/lib/github"
)

type ProjCommitsHandler struct {
	ac  acl.AccessControl
	gcl githublib.Client
	ecl *etcd.Client
}

func (h ProjCommitsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, projName string) {
	c, err := goship.ParseETCD(h.ecl)
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
	fp := acl.ReadableProjects(h.ac, []goship.Project{*proj}, u)
	p, err := retrieveCommits(h.gcl, h.ac, r, fp[0], c.DeployUser)
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
