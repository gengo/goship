package github

import (
	"testing"

	"github.com/gengo/goship/lib/config"
	"github.com/gengo/goship/lib/revision"
)

func TestSourceDiffURL(t *testing.T) {
	var ctl control
	for _, tt := range []struct {
		p        config.Project
		from, to revision.Revision
		want     string
	}{
		{
			p: config.Project{
				Name:      "test project",
				RepoOwner: "foo",
				RepoName:  "test",
			},
			from: "abc123",
			to:   "abc123",
			want: "",
		},
		{
			p: config.Project{
				Name:      "test project",
				RepoOwner: "foo",
				RepoName:  "test",
			},
			from: "abc123",
			to:   "abc456",
			want: "https://github.com/foo/test/compare/abc123...abc456",
		},
	} {
		if got := ctl.SourceDiffURL(tt.p, tt.from, tt.to); got != tt.want {
			t.Errorf("ctl.SourceDiffURL(%#v, %q, %q) = %q; want %q", tt.p, tt.from, tt.to, got, tt.want)
		}
	}
}
