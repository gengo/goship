package commits

import (
	"github.com/gengo/goship/lib/revision"
)

// environment describes the latest deployment status of a project in an environment.
type environment struct {
	// Name is the name of the environment
	Name string `json:"name"`
	sourceStatus
	Comment string `json:"comment"`
	// Locked is true iff the project is not ready for deployment.
	Locked bool `json:"isLocked"`
	// Deployments are per-host status of deployments
	Deployments []deployStatus `json:"deployments"`
}

// sourceStatus describes a latest deployable revision of a project
type sourceStatus struct {
	// Revision is the unique identifier of the revision
	Revision      revision.Revision `json:"latestDeployable"`
	ShortRevision revision.Revision `json:"shortLatestDeployable"`
	// SourceCodeRevision is a revision in a source code management system corresponding to the revision.
	// SourceCodeRevision can be equal to Revision if the underlying revision control system itself is
	// a soruce code management system,
	SourceCodeRevision revision.Revision `json:"sourceCodeRevison"`
}

// deployStatus describes a latest deployed revision of a project in a host
type deployStatus struct {
	// HostName is the name of the host
	HostName string `json:"hostname"`
	// Revision is the unique identifier of the revision
	Revision      revision.Revision `json:"revision"`
	ShortRevision revision.Revision `json:"shortRevision"`
	// Revision URL is an URL to a human-readable resource which describes the revision
	RevisionURL string `json:"revisionURL"`
	// SourceCodeRevision is a revision in a source code management system corresponding to the revision.
	// SourceCodeRevision can be equal to Revision if the underlying revision control system itself is
	// a soruce code management system,
	SourceCodeRevision revision.Revision `json:"sourceCodeRevison"`
	// SourceCodeDiffURL is an URL to a human-readable resource which describes difference between
	// the latest deployable source code and SourceCodeRevision.
	SourceCodeDiffURL string `json:"sourceCodeDiffURL"`
}
