package revision

import (
	"github.com/gengo/goship/lib/config"
	"golang.org/x/net/context"
)

// Revision is a revision of a project to be deployed.
type Revision string

func (r Revision) Short() Revision {
	if len(r) <= 7 {
		return r
	}
	return r[:7]
}

// Control is an abstraction of revision control systems
type Control interface {
	Latest(ctx context.Context, owner, repo, ref string) (rev Revision, srcRev Revision, err error)
	LatestDeployed(ctx context.Context, host, repoPath string) (rev Revision, srcRev Revision, err error)
	RevisionURL(p config.Project, rev Revision) string
	SourceDiffURL(p config.Project, from, to Revision) string
}
