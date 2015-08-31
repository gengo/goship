package main

import (
	"html/template"
	"net/http"
	"os"
	"sort"

	"github.com/coreos/go-etcd/etcd"
	"github.com/gengo/goship/lib/acl"
	"github.com/gengo/goship/lib/auth"
	"github.com/gengo/goship/lib/config"
	helpers "github.com/gengo/goship/lib/view-helpers"
	"github.com/gengo/goship/plugins/plugin"
	"github.com/golang/glog"
)

// HomeHandler is the main home screen
type HomeHandler struct {
	ac     acl.AccessControl
	ecl    *etcd.Client
	assets helpers.Assets
}

func (h HomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := config.Load(h.ecl)
	if err != nil {
		glog.Errorf("Failed to Parse to ETCD data %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	u, err := auth.CurrentUser(r)
	if err != nil {
		glog.Errorf("Failed to get current user: %v", err)
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	t, err := template.New("index.html").ParseFiles("templates/index.html", "templates/base.html")
	if err != nil {
		glog.Errorf("Failed to parse template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	projs := acl.ReadableProjects(h.ac, c.Projects, u)

	sort.Sort(ByName(c.Projects))

	// columns maps a plugin name to a list of columns
	columns := make(map[string][]plugin.Column)
	for _, pl := range plugin.Plugins {
		for _, p := range c.Projects {
			cols, err := pl.Apply(p)
			if err != nil {
				glog.Errorf("Failed to apply plugin: %s", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			columns[p.Name] = append(columns[p.Name], cols...)
		}
	}
	js, css := h.assets.Templates()
	gt := os.Getenv(gitHubAPITokenEnvVar)
	pt := c.Pivotal.Token

	params := map[string]interface{}{
		"Javascript":        js,
		"Stylesheet":        css,
		"Projects":          projs,
		"PluginColumns":     columns,
		"User":              u,
		"Page":              "home",
		"ConfirmDeployFlag": *confirmDeployFlag,
		"GithubToken":       gt,
		"PivotalToken":      pt,
	}
	helpers.RespondWithTemplate(w, "text/html", t, "base", params)
}

// ByName is the interface for sorting projects
type ByName []config.Project

func (slice ByName) Len() int           { return len(slice) }
func (slice ByName) Less(i, j int) bool { return slice[i].Name < slice[j].Name }
func (slice ByName) Swap(i, j int)      { slice[i], slice[j] = slice[j], slice[i] }
