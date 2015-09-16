package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"math"
	"net/http"
	"path"
	"sort"
	"time"

	"github.com/gengo/goship/lib/auth"
	"github.com/gengo/goship/lib/config"
	"github.com/gengo/goship/lib/revision"
	helpers "github.com/gengo/goship/lib/view-helpers"
	"github.com/golang/glog"
)

// DeployLogHandler shows data about the environment including the deploy log.
type DeployLogHandler struct {
	assets helpers.Assets
}

func (h DeployLogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, fullEnv string, environment config.Environment, projectName string) {
	u, err := auth.CurrentUser(r)
	if err != nil {
		glog.Errorf("Failed to get current user: %v", err)
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	d, err := readEntries(fullEnv)
	if err != nil {
		glog.Errorf("Failed to read entries: %v", err)
	}
	t, err := template.New("deploy_log.html").ParseFiles("templates/deploy_log.html", "templates/base.html")
	if err != nil {
		glog.Errorf("Failed to parse templates: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for i := range d {
		d[i].FormattedTime = formatTime(d[i].Time)
	}
	sort.Sort(ByTime(d))
	js, css := h.assets.Templates()

	params := map[string]interface{}{
		"Javascript":  js,
		"Stylesheet":  css,
		"Deployments": d,
		"User":        u,
		"Env":         fullEnv,
		"Environment": environment,
		"ProjectName": projectName,
	}
	helpers.RespondWithTemplate(w, "text/html", t, "base", params)
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

func readEntries(env string) ([]DeployLogEntry, error) {
	var d []DeployLogEntry
	b, err := ioutil.ReadFile(path.Join(*dataPath, env+".json"))
	if err != nil {
		return d, err
	}
	if len(b) == 0 {
		glog.Errorf("No deploy logs found for: %s", env)
		return []DeployLogEntry{}, nil
	}
	err = json.Unmarshal(b, &d)
	if err != nil {
		return d, err
	}

	return d, nil
}

type RevRange struct {
	From revision.Revision `json:"from"`
	To   revision.Revision `json:"to"`
}

type DeployLogEntry struct {
	Range         RevRange `json:"range"`
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
