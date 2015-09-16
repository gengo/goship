package gcr

import (
	"encoding/json"
	"fmt"
	"net/http"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// Scope is the OAuth2 scope necessary to access to Google Container Registry.
	// See also https://cloud.google.com/storage/docs/authentication#oauth
	Scope = "https://www.googleapis.com/auth/devstorage.read_only"
)

var (
	tokenSource oauth2.TokenSource
)

// Initialize initializes the package with a TokenSource to access to Google Container Registry
// TODO(yugui) Behave on behalf of the current user instead of service account.
func Initialize(ts oauth2.TokenSource) {
	tokenSource = ts
}

// fetchV1Manifest implements a client of Docker Registry API v1.
// https://docs.docker.com/v1.6/reference/api/registry_api/
//
// It is good enough to fetch image metadata (manifest) from Google Container Registry,
// but it is not a generic implementation of the API.
func fetchV1Manifest(ctx context.Context, img Name) (docker.Image, error) {
	if img.Registry != "gcr.io" {
		return docker.Image{}, fmt.Errorf("not Google Container Registry but %q", img.Registry)
	}

	ts := tokenSource
	if ts == nil {
		var err error
		ts, err = google.DefaultTokenSource(ctx, Scope)
		if err != nil {
			glog.Errorf("Failed to initialize default token source: %v", err)
			return docker.Image{}, err
		}
	}
	tok, err := ts.Token()
	if err != nil {
		glog.Errorf("Failed to fetch oauth2 access token: %v", err)
		return docker.Image{}, err
	}

	dockerToken, err := fetchToken(img, tok)
	if err != nil {
		glog.Errorf("Failed to fetch docker access token: %v", err)
		return docker.Image{}, err
	}
	id, err := fetchImageID(img, dockerToken)
	if err != nil {
		glog.Errorf("Failed to resolve image ID of %s : %v", img, err)
		return docker.Image{}, err
	}
	manifest, err := fetchManifest(id, dockerToken)
	if err != nil {
		glog.Errorf("Failed to fetch manifest of %s (%s): %v", img, id, err)
		return docker.Image{}, err
	}
	return manifest, nil
}

func fetchToken(img Name, tok *oauth2.Token) (string, error) {
	url := fmt.Sprintf("https://gcr.io/v1/repositories/%s/%s/images", img.NS, img.Repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth("_token", tok.AccessToken)
	req.Header.Set("X-Docker-Token", "true")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if code := resp.StatusCode; code < http.StatusOK || http.StatusMultipleChoices <= code {
		return "", fmt.Errorf("Unexpected HTTP status %d from %s", code, req.URL)
	}
	return resp.Header.Get("X-Docker-Token"), nil
}

func fetchImageID(img Name, token string) (string, error) {
	tag := img.Tag
	if tag == "" {
		tag = "latest"
	}
	url := fmt.Sprintf("https://gcr.io/v1/repositories/%s/%s/tags/%s", img.NS, img.Repo, tag)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Token %s", token))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if code := resp.StatusCode; code < http.StatusOK || http.StatusMultipleChoices <= code {
		return "", fmt.Errorf("Unexpected HTTP status %d from %s", code, req.URL)
	}
	var id string
	if err := json.NewDecoder(resp.Body).Decode(&id); err != nil {
		return "", err
	}
	return id, nil
}

func fetchManifest(id string, token string) (docker.Image, error) {
	url := fmt.Sprintf("https://gcr.io/v1/images/%s/json", id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return docker.Image{}, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Token %s", token))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return docker.Image{}, err
	}
	defer resp.Body.Close()
	if code := resp.StatusCode; code < http.StatusOK || http.StatusMultipleChoices <= code {
		return docker.Image{}, fmt.Errorf("Unexpected HTTP status %d from %s", code, req.URL)
	}

	var manifest docker.Image
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return docker.Image{}, err
	}
	return manifest, nil
}
