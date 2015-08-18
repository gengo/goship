package github

import (
	"fmt"
	"strings"

	goship "github.com/gengo/goship/lib"
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
func (c control) Latest(ctx context.Context, owner, repo, ref string) (revision.Revision, error) {
	opts := &github.CommitsListOptions{SHA: ref}
	commits, _, err := c.gcl.ListCommits(owner, repo, opts)
	if err != nil {
		glog.Errorf("Failed to get commits from GitHub: %v", err)
		return "", err
	}
	if len(commits) == 0 {
		glog.Errorf("No commits in branch %s of %s/%s", ref, owner, repo)
		return "", fmt.Errorf("no commits in the branch %s", ref)
	}
	return revision.Revision(*commits[0].SHA), nil
}

// LatestDeployed returns the latest commit deployed into the host.
func (c control) LatestDeployed(ctx context.Context, host goship.Host, repoPath string) (revision.Revision, error) {
	hostname := host.URI
	cmd := fmt.Sprintf("git --git-dir=%s rev-parse HEAD", repoPath)
	buf, err := c.ssh.Output(ctx, hostname, cmd)
	if err != nil {
		glog.Errorf("Failed to get latest deployed commit from %s:%s : %v", host.URI, repoPath, err)
		return "", err
	}
	sha1hash := strings.TrimSpace(string(buf))
	return revision.Revision(sha1hash), nil
}
