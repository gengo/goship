package gcr

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/gengo/goship/lib/config"
	"github.com/gengo/goship/lib/revision"
	"github.com/gengo/goship/lib/ssh"
	"github.com/golang/glog"
	"golang.org/x/net/context"
)

const (
	srcRevAttr = "source-revision"
)

type control struct {
	revision.SourceControl
	dcl *docker.Client
	ssh ssh.SSH
}

// New returns a new revision.Control which accesses to revisions of Docker images.
func New(srcCtl revision.SourceControl, dcl *docker.Client, ssh ssh.SSH) revision.Control {
	return control{
		SourceControl: srcCtl,
		dcl:           dcl,
		ssh:           ssh,
	}
}

func imgName(proj config.Project, env config.Environment) Name {
	name := Name{
		Registry: proj.RepoOwner,
		NS:       path.Dir(proj.RepoName),
		Repo:     path.Base(proj.RepoName),
		Tag:      env.Branch,
	}
	if !strings.Contains(proj.RepoName, "/") {
		name.NS = ""
	}
	return name
}

func (c control) Latest(ctx context.Context, proj config.Project, env config.Environment) (rev, srcRev revision.Revision, err error) {
	name := imgName(proj, env)
	glog.V(1).Infof("fetching manifest of %s from registry", name)
	img, err := fetchV1Manifest(ctx, name)
	if err != nil {
		return "", "", err
	}
	glog.V(2).Infof("%s in registry => %s", name, img.ID)
	return revision.Revision(img.ID), revision.Revision(img.Config.Labels[srcRevAttr]), nil
}

func (c control) LatestDeployed(ctx context.Context, hostname string, proj config.Project, env config.Environment) (rev, srcRev revision.Revision, err error) {
	name := imgName(proj, env)
	glog.V(1).Infof("fetching manifest of %s on %s", name, hostname)
	cmd := fmt.Sprintf("sudo docker inspect %s", name)
	buf, err := c.ssh.Output(ctx, hostname, cmd)
	if err != nil {
		glog.Errorf("Failed to inspect latest deployed image %s on %s: %v", name, hostname, err)
		return "", "", err
	}

	var imgs []docker.Image
	if err := json.Unmarshal(buf, &imgs); err != nil {
		return "", "", err
	}
	if len(imgs) == 0 {
		return "", "", fmt.Errorf("no such image %s on %s", name, hostname)
	}
	img := imgs[0]
	glog.V(2).Infof("%s on %s => %s", name, hostname, img.ID)
	return revision.Revision(img.ID), revision.Revision(img.Config.Labels[srcRevAttr]), nil
}

func (c control) RevisionURL(p config.Project, rev revision.Revision) string {
	return ""
}
