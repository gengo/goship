package main

import (
	"log"
	"net/http"

	"github.com/coreos/go-etcd/etcd"
	goship "github.com/gengo/goship/lib"
)

// CommentHandler allows you to update a comment on an environment
// i.e. http://127.0.0.1:8000/comment?environment=staging&project=admin&comment=DONOTDEPLOYPLEASE!
func CommentHandler(w http.ResponseWriter, r *http.Request) {
	c := etcd.NewClient([]string{*ETCDServer})
	p := r.FormValue("project")
	env := r.FormValue("environment")
	comment := r.FormValue("comment")
	err := goship.SetComment(c, p, env, comment)
	if err != nil {
		log.Println("ERROR: ", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
