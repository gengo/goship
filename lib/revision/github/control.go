package github

import (
	"fmt"
	"strings"

	"github.com/gengo/goship/lib/config"
	githublib "github.com/gengo/goship/lib/github"
	"github.com/gengo/goship/lib/revision"
	"github.com/gengo/goship/lib/ssh"
	"github.com/golang/glog"
	"github.com/google/go-github/github"
	"golang.org/x/net/context"
)

type control struct {
	gcl githublib.Client
	ssh ssh.SSH
}

// New returns a new git-based implementation of revision.Control
func New(gcl githublib.Client, ssh ssh.SSH) revision.Control {
	return control{gcl: gcl, ssh: ssh}
}

// Latest returns the latest commit in the given reference.
func (c control) Latest(ctx context.Context, proj config.Project, env config.Environment) (rev, srcRev revision.Revision, err error) {
	owner, repo, ref := proj.RepoOwner, proj.RepoName, env.Branch
	opts := &github.CommitsListOptions{SHA: ref}
	commits, _, err := c.gcl.ListCommits(owner, repo, opts)
	if err != nil {
		glog.Errorf("Failed to get commits from GitHub: %v", err)
		return "", "", err
	}
	if len(commits) == 0 {
		glog.Errorf("No commits in branch %s of %s/%s", ref, owner, repo)
		return "", "", fmt.Errorf("no commits in the branch %s", ref)
	}
	rev = revision.Revision(*commits[0].SHA)
	return rev, rev, nil
}

// LatestDeployed returns the latest commit deployed into the host.
func (c control) LatestDeployed(ctx context.Context, hostname string, proj config.Project, env config.Environment) (rev, srcRev revision.Revision, err error) {
	cmd := fmt.Sprintf("git --git-dir=%s rev-parse HEAD", env.RepoPath)
	if proj.HostType == config.HostTypeK8s {
		selector := proj.Name
		if proj.K8sSelector != "" {
			selector = proj.K8sSelector
		}
		cmd = fmt.Sprintf("kubectl get %s -L git_version --no-headers -l name=%s | awk '{printf $NF}'", proj.K8sResource, selector)
	}
	buf, err := c.ssh.Output(ctx, hostname, cmd)
	if err != nil {
		glog.Errorf("Failed to get latest deployed commit from %s:%s : %v", hostname, env.RepoPath, err)
		return "", "", err
	}
	rev = revision.Revision(strings.TrimSpace(string(buf)))
	return rev, rev, nil
}

func (c control) RevisionURL(p config.Project, rev revision.Revision) string {
	return fmt.Sprintf("https://github.com/%s/%s/commit/%s", p.RepoOwner, p.RepoName, rev)
}

func (c control) SourceDiffURL(p config.Project, from, to revision.Revision) string {
	if from == to {
		return ""
	}
	repo := p.SourceRepo()
	return fmt.Sprintf("https://github.com/%s/%s/compare/%s...%s", repo.RepoOwner, repo.RepoName, from, to)
}

func (c control) SourceRevMessage(ctx context.Context, p config.Project, rev revision.Revision) (string, error) {
	repo := p.SourceRepo()
	commit, _, err := c.gcl.GetCommit(repo.RepoOwner, repo.RepoName, string(rev))
	if err != nil {
		return "", err
	}
	if commit.Message == nil {
		return "", nil
	}
	return *commit.Message, nil
}
