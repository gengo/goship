package comment

import (
	"net/http"

	"github.com/coreos/go-etcd/etcd"
	"github.com/gengo/goship/lib/config"
	"github.com/golang/glog"
)

// CommentHandler allows you to update a comment on an environment
// i.e. http://127.0.0.1:8000/comment?environment=staging&project=admin&comment=DONOTDEPLOYPLEASE!
type handler struct {
	ecl *etcd.Client
}

func New(ecl *etcd.Client) http.Handler {
	return handler{ecl: ecl}
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.FormValue("project")
	env := r.FormValue("environment")
	comment := r.FormValue("comment")
	err := config.SetComment(h.ecl, p, env, comment)
	if err != nil {
		glog.Errorf("Failed to store comment for project=%s env=%s: %v", p, env, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
