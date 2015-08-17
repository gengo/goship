package commits

import (
	"encoding/json"
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
	c, err := config.ParseETCD(h.ecl)
	if err != nil {
		glog.Errorf("Parsing etc: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	u, err := auth.CurrentUser(r)
	if err != nil {
		glog.Errorf("Failed to get current user: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	proj, err := config.ProjectFromName(c.Projects, projName)
	if err != nil {
		glog.Errorf("Failed to get project from name: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fp := acl.ReadableProjects(h.ac, []config.Project{*proj}, u)
	if len(fp) == 0 {
		http.Error(w, "Permission denied", http.StatusForbidden)
		return
	}

	p, err := h.retrieveCommits(ctx, fp[0], c.DeployUser)
	if err != nil {
		glog.Errorf("Failed to retrieve commits: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p = addPermissionComment(h.ac, p, u)

	j, err := json.Marshal(p)
	if err != nil {
		glog.Errorf("Failed to marshal response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(j)
	if err != nil {
		glog.Errorf("Failed to send response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h handler) retrieveCommits(ctx context.Context, project config.Project, deployUser string) (config.Project, error) {
	s, err := ssh.WithPrivateKeyFile(deployUser, h.sshKeyPath)
	if err != nil {
		return config.Project{}, err
	}
	c := revision.Control(githubrev.New(h.gcl, s))
	var wg sync.WaitGroup
	for i, environment := range project.Environments {
		for j, host := range environment.Hosts {
			wg.Add(1)
			go func(i, j int, host config.Host, repoPath string) {
				defer wg.Done()
				commit, err := c.LatestDeployed(ctx, host, repoPath)
				if err != nil {
					project.Environments[i].Hosts[j].LatestCommit = ""
					return
				}
				project.Environments[i].Hosts[j].LatestCommit = string(commit)
			}(i, j, host, environment.RepoPath)
		}
		wg.Add(1)
		go func(i int, ref string) {
			defer wg.Done()
			commit, err := c.Latest(ctx, project.RepoOwner, project.RepoName, ref)
			if err != nil {
				project.Environments[i].LatestGitHubCommit = ""
				return
			}
			project.Environments[i].LatestGitHubCommit = string(commit)
		}(i, environment.Branch)
	}
	wg.Wait()

	for i, e := range project.Environments {
		project.Environments[i] = e
		for j, host := range e.Hosts {
			host.GitHubCommitURL = host.LatestGitHubCommitURL(project)
			host.GitHubDiffURL = host.LatestGitHubDiffURL(project, e)
			host.ShortCommitHash = host.LatestShortCommitHash()
			project.Environments[i].Hosts[j] = host
		}
	}
	return project, nil
}

func addPermissionComment(ac acl.AccessControl, p config.Project, u auth.User) config.Project {
	for i, e := range p.Environments {
		// If the repo isn't already locked.. lock it if the user doesnt have permission
		// and add to the comments
		if e.IsLocked {
			if p.Environments[i].Comment != "" {
				p.Environments[i].Comment = p.Environments[i].Comment + " | "
			}
			p.Environments[i].Comment = p.Environments[i].Comment + "repo is locked."
			continue
		}

		locked := !ac.Deployable(p.RepoOwner, p.RepoName, u.Name)
		p.Environments[i].IsLocked = locked
		// Add a line break if there is already a comment
		if p.Environments[i].Comment != "" {
			p.Environments[i].Comment = p.Environments[i].Comment + " | "
		}
		if locked {
			p.Environments[i].Comment = p.Environments[i].Comment + "you do not have permission to deploy "
		}
	}
	return p
}
