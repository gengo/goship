package main

import (
	"code.google.com/p/goauth2/oauth"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/gorilla/mux"
	"html/template"
	"log"
	"net/http"
	"os"
)

//  Will return the latest commit hash, waiting on https://github.com/google/go-github/pull/49
func latestCommit(c *github.Client, repoName string) *github.Repository {
	repo, _, err := c.Repositories.Get("gengo", repoName)
	if err != nil {
		log.Panic(err)
	}
	return repo
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("templates/index.html")
	t.Execute(w, nil)
}

func main() {
	githubToken := os.Getenv("GITHUB_API_TOKEN")
	t := &oauth.Transport{
		Token: &oauth.Token{AccessToken: githubToken},
	}
	client := github.NewClient(t.Client())
	fmt.Println(client)

	r := mux.NewRouter()
	r.HandleFunc("/", HomeHandler)
	http.ListenAndServe(":8080", r)
}
