package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"

	"github.com/coreos/go-etcd/etcd"
	goship "github.com/gengo/goship/lib"
	"github.com/gengo/goship/lib/acl"
	"github.com/gengo/goship/lib/auth"
	helpers "github.com/gengo/goship/lib/view-helpers"
	"github.com/gengo/goship/plugins/plugin"
)

// HomeHandler is the main home screen
type HomeHandler struct {
	ac     acl.AccessControl
	ecl    *etcd.Client
	assets helpers.Assets
}

func (h HomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := goship.ParseETCD(h.ecl)
	if err != nil {
		log.Printf("Failed to Parse to ETCD data %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	u, err := auth.CurrentUser(r)
	if err != nil {
		log.Println("Failed to get User! ")
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	t, err := template.New("index.html").ParseFiles("templates/index.html", "templates/base.html")
	if err != nil {
		log.Printf("Failed to parse template: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.Projects = acl.ReadableProjects(h.ac, c.Projects, u)

	sort.Sort(ByName(c.Projects))

	// apply each plugin
	for _, pl := range plugin.Plugins {
		err := pl.Apply(c)
		if err != nil {
			log.Printf("Failed to apply plugin: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
	js, css := h.assets.Templates()
	gt := os.Getenv(gitHubAPITokenEnvVar)
	pt := c.Pivotal.Token

	t.ExecuteTemplate(w, "base", map[string]interface{}{"Javascript": js, "Stylesheet": css, "Projects": c.Projects, "User": u, "Page": "home", "ConfirmDeployFlag": *confirmDeployFlag, "GithubToken": gt, "PivotalToken": pt})
}

// ByName is the interface for sorting projects
type ByName []goship.Project

func (slice ByName) Len() int           { return len(slice) }
func (slice ByName) Less(i, j int) bool { return slice[i].Name < slice[j].Name }
func (slice ByName) Swap(i, j int)      { slice[i], slice[j] = slice[j], slice[i] }
