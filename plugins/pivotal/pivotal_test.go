package pivotal

import (
	"html/template"
	"testing"

	"github.com/gengo/goship/lib/config"
)

func TestRenderDetail(t *testing.T) {
	c := StoryColumn{}
	got, err := c.RenderDetail()
	if err != nil {
		t.Errorf(err.Error())
	}
	want := template.HTML(`<td class="story"></td>`)
	if want != got {
		t.Errorf("Want %#v, got %#v", want, got)
	}
}

func TestRenderHeader(t *testing.T) {
	c := StoryColumn{}
	got, err := c.RenderHeader()
	if err != nil {
		t.Errorf(err.Error())
	}
	want := template.HTML(`<th style="min-width: 200px;">Stories</th>`)
	if want != got {
		t.Errorf("Want %#v, got %#v", want, got)
	}
}

func TestApply(t *testing.T) {
	p := &PivotalPlugin{}
	proj := config.Project{
		Repo: config.Repo{
			RepoName:  "test_project",
			RepoOwner: "test",
		},
	}
	cols, err := p.Apply(proj)
	if err != nil {
		t.Errorf("p.Apply(%#v) failed with %v; want success", proj, err)
	}
	if got, want := len(cols), 1; got != want {
		t.Errorf("len(cols) = %d; want %d", got, want)
		return
	}
	if _, ok := cols[0].(StoryColumn); !ok {
		t.Errorf("cols[0] = %#v; want a StoryColumn", cols[0])
	}
}
