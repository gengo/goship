// Copyright 2013 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

func TestOrganizationsService_ListMembers(t *testing.T) {
	setup()
	defer teardown()

	mux.HandleFunc("/orgs/o/members", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"id":1}]`)
	})

	members, _, err := client.Organizations.ListMembers("o", false)
	if err != nil {
		t.Errorf("Organizations.ListMembers returned error: %v", err)
	}

	want := []User{{ID: Int(1)}}
	if !reflect.DeepEqual(members, want) {
		t.Errorf("Organizations.ListMembers returned %+v, want %+v", members, want)
	}
}

func TestOrganizationsService_ListMembers_invalidOrg(t *testing.T) {
	_, _, err := client.Organizations.ListMembers("%", false)
	testURLParseError(t, err)
}

func TestOrganizationsService_ListMembers_public(t *testing.T) {
	setup()
	defer teardown()

	mux.HandleFunc("/orgs/o/public_members", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"id":1}]`)
	})

	members, _, err := client.Organizations.ListMembers("o", true)
	if err != nil {
		t.Errorf("Organizations.ListMembers returned error: %v", err)
	}

	want := []User{{ID: Int(1)}}
	if !reflect.DeepEqual(members, want) {
		t.Errorf("Organizations.ListMembers returned %+v, want %+v", members, want)
	}
}

func TestOrganizationsService_IsMember(t *testing.T) {
	setup()
	defer teardown()

	mux.HandleFunc("/orgs/o/members/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNoContent)
	})

	member, _, err := client.Organizations.IsMember("o", "u")
	if err != nil {
		t.Errorf("Organizations.IsMember returned error: %v", err)
	}
	if want := true; member != want {
		t.Errorf("Organizations.IsMember returned %+v, want %+v", member, want)
	}
}

// ensure that a 404 response is interpreted as "false" and not an error
func TestOrganizationsService_IsMember_notMember(t *testing.T) {
	setup()
	defer teardown()

	mux.HandleFunc("/orgs/o/members/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	member, _, err := client.Organizations.IsMember("o", "u")
	if err != nil {
		t.Errorf("Organizations.IsMember returned error: %+v", err)
	}
	if want := false; member != want {
		t.Errorf("Organizations.IsMember returned %+v, want %+v", member, want)
	}
}

// ensure that a 400 response is interpreted as an actual error, and not simply
// as "false" like the above case of a 404
func TestOrganizationsService_IsMember_error(t *testing.T) {
	setup()
	defer teardown()

	mux.HandleFunc("/orgs/o/members/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		http.Error(w, "BadRequest", http.StatusBadRequest)
	})

	member, _, err := client.Organizations.IsMember("o", "u")
	if err == nil {
		t.Errorf("Expected HTTP 400 response")
	}
	if want := false; member != want {
		t.Errorf("Organizations.IsMember returned %+v, want %+v", member, want)
	}
}

func TestOrganizationsService_IsMember_invalidOrg(t *testing.T) {
	_, _, err := client.Organizations.IsMember("%", "u")
	testURLParseError(t, err)
}

func TestOrganizationsService_IsPublicMember(t *testing.T) {
	setup()
	defer teardown()

	mux.HandleFunc("/orgs/o/public_members/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNoContent)
	})

	member, _, err := client.Organizations.IsPublicMember("o", "u")
	if err != nil {
		t.Errorf("Organizations.IsPublicMember returned error: %v", err)
	}
	if want := true; member != want {
		t.Errorf("Organizations.IsPublicMember returned %+v, want %+v", member, want)
	}
}

// ensure that a 404 response is interpreted as "false" and not an error
func TestOrganizationsService_IsPublicMember_notMember(t *testing.T) {
	setup()
	defer teardown()

	mux.HandleFunc("/orgs/o/public_members/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	member, _, err := client.Organizations.IsPublicMember("o", "u")
	if err != nil {
		t.Errorf("Organizations.IsPublicMember returned error: %v", err)
	}
	if want := false; member != want {
		t.Errorf("Organizations.IsPublicMember returned %+v, want %+v", member, want)
	}
}

// ensure that a 400 response is interpreted as an actual error, and not simply
// as "false" like the above case of a 404
func TestOrganizationsService_IsPublicMember_error(t *testing.T) {
	setup()
	defer teardown()

	mux.HandleFunc("/orgs/o/public_members/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		http.Error(w, "BadRequest", http.StatusBadRequest)
	})

	member, _, err := client.Organizations.IsPublicMember("o", "u")
	if err == nil {
		t.Errorf("Expected HTTP 400 response")
	}
	if want := false; member != want {
		t.Errorf("Organizations.IsPublicMember returned %+v, want %+v", member, want)
	}
}

func TestOrganizationsService_IsPublicMember_invalidOrg(t *testing.T) {
	_, _, err := client.Organizations.IsPublicMember("%", "u")
	testURLParseError(t, err)
}

func TestOrganizationsService_RemoveMember(t *testing.T) {
	setup()
	defer teardown()

	mux.HandleFunc("/orgs/o/members/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	_, err := client.Organizations.RemoveMember("o", "u")
	if err != nil {
		t.Errorf("Organizations.RemoveMember returned error: %v", err)
	}
}

func TestOrganizationsService_RemoveMember_invalidOrg(t *testing.T) {
	_, err := client.Organizations.RemoveMember("%", "u")
	testURLParseError(t, err)
}
