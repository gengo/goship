package travis

import (
	"errors"
	"html/template"
	"testing"

	"github.com/coreos/go-etcd/etcd"
	goship "github.com/gengo/goship/lib"
)

type tokenMockClient struct {
	Token string
}

func (*tokenMockClient) Set(s, c string, x uint64) (*etcd.Response, error) {
	return nil, nil
}

//Mock calls to ETCD here. Each etcd Response should return the structs you need.
func (*tokenMockClient) Get(s string, t bool, x bool) (*etcd.Response, error) {
	m := make(map[string]*etcd.Response)
	m["/projects/test_private/travis_token"] = &etcd.Response{
		Action: "Get",
		Node: &etcd.Node{
			Key: "/projects/test_private/travis_token", Value: "test_token",
		},
	}
	mockResponse, ok := m[s]
	if !ok {
		return nil, errors.New("Key doesn't exist!")
	}
	return mockResponse, nil
}

func TestGetTokenPrivate(t *testing.T) {
	mockEtcd := &tokenMockClient{
		Token: "test_token",
	}
	p := goship.Project{
		Name: "test_private",
	}
	got := getToken(mockEtcd, p)
	want := mockEtcd.Token
	if got != want {
		t.Errorf("Want %#v, got %#v", want, got)
	}
}

func TestGetTokenPublic(t *testing.T) {
	p := goship.Project{
		Name: "test_public",
	}
	got := getToken(&tokenMockClient{}, p)
	want := ""
	if got != want {
		t.Errorf("Want %#v, got %#v", want, got)
	}
}

func TestRenderHeader(t *testing.T) {
	c := TravisColumn{
		Project:      "test_public",
		Token:        "",
		Organization: "test",
	}
	got, err := c.RenderHeader()
	if err != nil {
		t.Errorf(err.Error())
	}
	want := template.HTML("<th>Build Status</th>")
	if want != got {
		t.Errorf("Want %#v, got %#v", want, got)
	}
}

func TestRenderDetailPublic(t *testing.T) {
	c := TravisColumn{
		Project:      "test_public",
		Token:        "",
		Organization: "test",
	}
	got, err := c.RenderDetail()
	if err != nil {
		t.Errorf(err.Error())
	}
	want := template.HTML(`<td><a target=_blank href=https://travis-ci.org/test/test_public><img src=https://travis-ci.org/test/test_public.svg?branch=master onerror='this.style.display = "none"'></img></a></td>`)
	if want != got {
		t.Errorf("Want %#v, got %#v", want, got)
	}
}

func TestRenderDetailPrivate(t *testing.T) {
	c := TravisColumn{
		Project:      "test_private",
		Token:        "test_token",
		Organization: "test",
	}
	got, err := c.RenderDetail()
	if err != nil {
		t.Errorf(err.Error())
	}
	want := template.HTML(`<td><a target=_blank href=https://magnum.travis-ci.com/test/test_private><img src=https://magnum.travis-ci.com/test/test_private.svg?token=test_token&branch=master onerror='this.style.display = "none"'></img></a></td>`)
	if want != got {
		t.Errorf("Want %#v, got %#v", want, got)
	}
}

func TestApply(t *testing.T) {
	p := &TravisPlugin{}
	config := goship.Config{
		Projects: []goship.Project{
			goship.Project{
				RepoName:  "test_project",
				RepoOwner: "test",
			},
		},
		ETCDClient: &tokenMockClient{},
	}
	err := p.Apply(config)
	if err != nil {
		t.Fatalf("Error applying plugin %v", err)
	}
	if len(config.Projects[0].PluginColumns) != 1 {
		t.Fatalf("Failed to add plugin column, PluginColumn len = %d", len(config.Projects[0].PluginColumns))
	}
	pl := config.Projects[0].PluginColumns[0]
	switch pl.(type) {
	case TravisColumn:
		break
	default:
		t.Errorf("Plugin is not correct type, type %T", pl)
	}
}
