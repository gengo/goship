package main

import (
	"html/template"
	"log"
	"net/http"

	"github.com/gengo/goship/lib/auth"
)

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
