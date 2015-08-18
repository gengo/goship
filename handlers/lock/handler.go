package lock

import (
	"net/http"

	"github.com/coreos/go-etcd/etcd"
	goship "github.com/gengo/goship/lib"
	"github.com/golang/glog"
)

// http://127.0.0.1:8000/lock?environment=staging&project=admin
func NewLock(ecl *etcd.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(ecl, w, r, true)
	})
}

func NewUnlock(ecl *etcd.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(ecl, w, r, false)
	})
}

// handler allows you to lock or unlock an environment
func handler(ecl *etcd.Client, w http.ResponseWriter, r *http.Request, lock bool) {
	p := r.FormValue("project")
	env := r.FormValue("environment")

	lockStr := "false"
	if lock {
		lockStr = "true"
	}
	err := goship.LockEnvironment(ecl, p, env, lockStr)
	if err != nil {
		glog.Errorf("Failed to lock/unlock project=%s env=%s: %v", p, env, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
