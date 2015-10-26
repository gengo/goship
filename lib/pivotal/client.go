package pivotal

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/golang/glog"
)

const (
	pivotalBaseURL = "https://www.pivotaltracker.com/services/v5/"
)

// Client is an interface for testability.
// It provides access to a subset of Pivotal APIs.
type Client interface {
	FindProjectForStory(id int) (int, error)
	AddLabel(id int, project int, label string) error
	AddComment(id int, project int, comment string) error
}

type pivClient struct {
	token string
}

// NewClient returns a new client of Pivotal APIs.
// "token" must be a valid Pivotal API access token
func NewClient(token string) Client {
	return pivClient{
		token: token,
	}
}

func (c pivClient) request(method string, endpoint string, form url.Values) ([]byte, error) {
	req, err := http.NewRequest(method, pivotalBaseURL+endpoint, nil)
	if err != nil {
		glog.Errorf("could not form get request to Pivotal: %v", err)
		return nil, err
	}
	if form != nil {
		req.URL.RawQuery = form.Encode()
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-TrackerToken", c.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		glog.Errorf("could not make put request to Pivotal: %v", err)
		return nil, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		m := fmt.Sprintf("bad status code returned by Pivotal: %s [%d] (%s)", resp.Status, resp.StatusCode, string(b))
		glog.Error(m)
		return nil, fmt.Errorf(m)
	}
	return b, nil
}

// FindProjectForStory returns the project id for a Pivotal story
func (c pivClient) FindProjectForStory(id int) (int, error) {
	b, err := c.request("GET", fmt.Sprintf("stories/%d", id), nil)
	if err != nil {
		return 0, err
	}
	var p struct {
		ProjectID int `json:"project_id"`
	}
	err = json.Unmarshal(b, &p)
	if err != nil {
		return 0, err
	}
	return p.ProjectID, nil
}

// AddLabel adds a label to a story
func (c pivClient) AddLabel(id int, project int, label string) error {
	p := url.Values{
		"name": []string{label},
	}
	_, err := c.request("POST", fmt.Sprintf("projects/%d/stories/%d/labels", project, id), p)
	return err
}

// AddComment posts a comment to a story.
func (c pivClient) AddComment(id int, project int, comment string) error {
	p := url.Values{
		"text": []string{comment},
	}
	_, err := c.request("POST", fmt.Sprintf("projects/%d/stories/%d/comments", project, id), p)
	return err
}
