package comment

import (
	"log"
	"net/http"

	"github.com/coreos/go-etcd/etcd"
	goship "github.com/gengo/goship/lib"
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
	err := goship.SetComment(h.ecl, p, env, comment)
	if err != nil {
		log.Println("ERROR: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
