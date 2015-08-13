package main

import (
	"log"
	"net/http"

	"github.com/coreos/go-etcd/etcd"
	goship "github.com/gengo/goship/lib"
)

// LockHandler allows you to lock an environment
// http://127.0.0.1:8000/lock?environment=staging&project=admin
func LockHandler(w http.ResponseWriter, r *http.Request) {
	c := etcd.NewClient([]string{*ETCDServer})
	p := r.FormValue("project")
	env := r.FormValue("environment")
	err := goship.LockEnvironment(c, p, env, "true")
	if err != nil {
		log.Println("ERROR: ", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
