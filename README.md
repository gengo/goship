GoShip
======

Deployment tool for deploying code to servers and seeing what is deployed.

Installation
------------

To clone this project:

    git clone git@github.com:gengo/goship.git

This is a Go project, so a prerequisite is to have a working Go environment and `GOPATH` set up.

Before you can use GoShip, you will need to export your Github token:

    export GITHUB_API_TOKEN="your-organization-github-token-here"

and also install the dependencies:

    go get ./...
