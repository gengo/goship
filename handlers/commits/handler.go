package commits

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/coreos/go-etcd/etcd"
	"github.com/gengo/goship/lib/acl"
	"github.com/gengo/goship/lib/auth"
	"github.com/gengo/goship/lib/config"
	githublib "github.com/gengo/goship/lib/github"
	"github.com/gengo/goship/lib/revision"
	githubrev "github.com/gengo/goship/lib/revision/github"
	"github.com/gengo/goship/lib/ssh"
	"github.com/golang/glog"
	"golang.org/x/net/context"
)

var (
	projectUnaccessible = errors.New("permission denied")
)

type handler struct {
	ac         acl.AccessControl
	ecl        *etcd.Client
	gcl        githublib.Client
	sshKeyPath string
}

// New returns a new http.Handler which serves latest revisions in deploy targets and the revision control system.
func New(ac acl.AccessControl, ecl *etcd.Client, gcl githublib.Client, sshKeyPath string) http.Handler {
	return handler{ac: ac, ecl: ecl, gcl: gcl, sshKeyPath: sshKeyPath}
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	components := strings.Split(r.URL.Path, "/")
	if len(components) != 3 || components[0] != "" || components[1] != "commits" {
		http.NotFound(w, r)
		return
	}
	projName := components[2]
	u, err := auth.CurrentUser(r)
	if err != nil {
		glog.Errorf("Failed to get current user: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	envs, err := h.fetchStatuses(ctx, projName, u)
	if err == projectUnaccessible {
		glog.Errorf("project %s is not accessible for %s", projName, u.Name)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	buf, err := json.Marshal(envs)
	if err != nil {
		glog.Errorf("Failed to marshal response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(buf); err != nil {
		glog.Errorf("Failed to send response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h handler) fetchStatuses(ctx context.Context, projName string, u auth.User) ([]environment, error) {
	p, deployUser, err := h.loadProject(projName, u)
	if err != nil {
		return nil, err
	}
	envs, err := h.retrieveCommits(ctx, p, deployUser)
	if err != nil {
		glog.Errorf("Failed to retrieve commits: %v", err)
		return nil, err
	}

	for i := range envs {
		env := &envs[i]
		locked, comments := func() (bool, []string) {
			var comments []string
			if c := env.Comment; c != "" {
				comments = append(comments, c)
			}
			if env.Locked {
				return true, append(comments, "repo is locked.")
			}
			repo := p.SourceRepo()
			if !h.ac.Deployable(repo.RepoOwner, repo.RepoName, u.Name) {
				return true, append(comments, "you do not have permission to deploy")
			}
			return false, comments
		}()
		env.Locked = locked
		env.Comment = strings.Join(comments, " | ")
	}

	return envs, nil
}

func (h handler) loadProject(projName string, u auth.User) (p config.Project, deployUser string, err error) {
	c, err := config.Load(h.ecl)
	if err != nil {
		glog.Errorf("Parsing etc: %v", err)
		return config.Project{}, "", err
	}
	p, err = config.ProjectFromName(c.Projects, projName)
	if err != nil {
		glog.Errorf("Failed to get project from name: %v", err)
		return config.Project{}, "", err
	}
	repo := p.SourceRepo()
	if !h.ac.Readable(repo.RepoOwner, repo.RepoName, u.Name) {
		return config.Project{}, "", projectUnaccessible
	}
	return p, c.DeployUser, nil

}

func (h handler) retrieveCommits(ctx context.Context, proj config.Project, deployUser string) ([]environment, error) {
	s, err := ssh.WithPrivateKeyFile(deployUser, h.sshKeyPath)
	if err != nil {
		return nil, err
	}
	c := revision.Control(githubrev.New(h.gcl, s))

	var wg sync.WaitGroup
	envs := make([]environment, len(proj.Environments))
	for i, e := range proj.Environments {
		envs[i] = environment{
			Name:        e.Name,
			Locked:      e.IsLocked,
			Deployments: make([]deployStatus, len(e.Hosts)),
		}
		env := &envs[i]

		for j, host := range e.Hosts {
			wg.Add(1)
			go func(st *deployStatus, host string, e config.Environment) {
				defer wg.Done()
				rev, srcRev, err := c.LatestDeployed(ctx, host, proj, e)
				if err != nil {
					st.Revision = ""
					return
				}
				st.Revision = rev
				st.ShortRevision = rev.Short()
				st.RevisionURL = c.RevisionURL(proj, rev)
				st.SourceCodeRevision = srcRev
			}(&env.Deployments[j], host, e)
		}
		wg.Add(1)
		go func(env *environment, e config.Environment) {
			defer wg.Done()
			rev, srcRev, err := c.Latest(ctx, proj, e)
			if err != nil {
				env.Revision = ""
				return
			}
			env.Revision = rev
			env.SourceCodeRevision = srcRev
			env.ShortRevision = rev.Short()
		}(env, e)
	}
	wg.Wait()

	for i := range envs {
		env := &envs[i]
		for j := range env.Deployments {
			d := &env.Deployments[j]
			d.SourceCodeDiffURL = c.SourceDiffURL(proj, d.SourceCodeRevision, env.SourceCodeRevision)
		}
	}
	return envs, nil
}
