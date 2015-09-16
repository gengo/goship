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

type SourceControl interface {
	SourceDiffURL(p config.Project, from, to Revision) string
	SourceRevMessage(ctx context.Context, p config.Project, rev Revision) (string, error)
}

// Control is an abstraction of revision control systems
type Control interface {
	SourceControl

	Latest(ctx context.Context, proj config.Project, env config.Environment) (rev, srcRev Revision, err error)
	LatestDeployed(ctx context.Context, hostname string, proj config.Project, env config.Environment) (rev, srcRev Revision, err error)
	RevisionURL(p config.Project, rev Revision) string
}
