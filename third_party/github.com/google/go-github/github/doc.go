// Copyright 2013 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package github provides a client for using the GitHub API.

Construct a new GitHub client, then use the various services on the client to
access different parts of the GitHub API. For example:

	client := github.NewClient(nil)

	// list all organizations for user "willnorris"
	orgs, _, err := client.Organizations.List("willnorris", nil)

Set optional parameters for an API method by passing an Options object.

	// list recently updated repositories for org "github"
	opt := &github.RepositoryListByOrgOptions{Sort: "updated"}
	repos, _, err := client.Repositories.ListByOrg("github", opt)

The services of a client divide the API into logical chunks and correspond to
the structure of the GitHub API documentation at
http://developer.github.com/v3/.

Authentication

The go-github library does not directly handle authentication. Instead, when
creating a new client, pass an http.Client that can handle authentication for
you. The easiest and recommended way to do this is using the goauth2 library,
but you can always use any other library that provides an http.Client. If you
have an OAuth2 access token (for example, a personal API token), you can use it
with the goauth2 using:

	import "code.google.com/p/goauth2/oauth"

	// simple OAuth transport if you already have an access token;
	// see goauth2 library for full usage
	t := &oauth.Transport{
		Token: &oauth.Token{AccessToken: "..."},
	}

	client := github.NewClient(t.Client())

	// list all repositories for the authenticated user
	repos, _, err := client.Repositories.List("", nil)

Note that when using an authenticated Client, all calls made by the client will
include the specified OAuth token. Therefore, authenticated clients should
almost never be shared between different users.

Rate Limiting

GitHub imposes a rate limit on all API clients.  Unauthenticated clients are
limited to 60 requests per hour, while authenticated clients can make up to
5,000 requests per hour.  To receive the higher rate limit when making calls
that are not issued on behalf of a user, use the
UnauthenticatedRateLimitedTransport.

The Rate field on a client tracks the rate limit information based on the most
recent API call.  This is updated on every call, but may be out of date if it's
been some time since the last API call and other clients have made subsequent
requests since then.  You can always call RateLimit() directly to get the most
up-to-date rate limit data for the client.

Learn more about GitHub rate limiting at
http://developer.github.com/v3/#rate-limiting.

Creating and Updating Resources

All structs for GitHub resources use pointer values for all non-repeated fields.
This allows distinguishing between unset fields and those set to a zero-value.
Helper functions have been provided to easily create these pointers for string,
bool, and int values.  For example:

	// create a new private repository named "foo"
	repo := &github.Repo{
		Name:    github.String("foo"),
		Private: github.Bool(true),
	}
	client.Repositories.Create("", repo)

Users who have worked with protocol buffers should find this pattern familiar.
*/
package github
