package revision

import (
	goship "github.com/gengo/goship/lib"
	"golang.org/x/net/context"
)

// Revision is a revision of a project to be deployed.
type Revision string

// Control is an abstraction of revision control systems
type Control interface {
	Latest(ctx context.Context, owner, repo, ref string) (Revision, error)
	LatestDeployed(ctx context.Context, host goship.Host, repoPath string) (Revision, error)
}
