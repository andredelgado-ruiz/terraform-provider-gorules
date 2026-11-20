package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoleClientGetPaginatesAndReportsMissing(t *testing.T) {
	var pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("missing bearer token")
		}
		pages = append(pages, r.URL.Query().Get("page"))
		switch r.URL.Query().Get("page") {
		case "1":
			fmt.Fprint(w, `{"results":[{"id":"a","name":"a"}],"paginate":{"total":2}}`)
		case "2":
			fmt.Fprint(w, `{"results":[{"id":"b","name":"b"}],"paginate":{"total":2}}`)
		default:
			t.Errorf("unexpected page %s", r.URL.Query().Get("page"))
		}
	}))
	defer srv.Close()
	c := &roleClient{baseURL: srv.URL, token: "tok"}

	item, err := c.get(context.Background(), "b")
	if err != nil || item.ID != "b" {
		t.Fatalf("expected role b on page 2, got %v, %v", item, err)
	}

	if _, err := c.get(context.Background(), "missing"); !errors.Is(err, errRoleNotFound) {
		t.Fatalf("expected errRoleNotFound, got %v", err)
	}
}

func TestRoleClientUpdatePreservesUsersAndFilePermissions(t *testing.T) {
	var put map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"results":[{"id":"r1","name":"old","allProjects":false,
				"users":[{"id":"u1","email":"u1@example.com"},{"id":"u2","email":"u2@example.com"}],
				"projectFilePermissions":[{"id":"f1","effect":"deny","actions":["delete"],"patterns":"secret/*"}]}],
				"paginate":{"total":1}}`)
		case http.MethodPut:
			raw, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(raw, &put); err != nil {
				t.Errorf("invalid body: %v", err)
			}
			fmt.Fprint(w, `{"id":"r1","name":"new","allProjects":false}`)
		}
	}))
	defer srv.Close()
	c := &roleClient{baseURL: srv.URL, token: "tok"}

	if _, err := c.update(context.Background(), "r1", roleWriteBody{Name: "new"}); err != nil {
		t.Fatal(err)
	}
	if got := string(put["users"]); got != `["u1","u2"]` {
		t.Errorf("users not preserved: %s", got)
	}
	var fp []map[string]any
	if err := json.Unmarshal(put["projectFilePermissions"], &fp); err != nil || len(fp) != 1 || fp[0]["patterns"] != "secret/*" {
		t.Errorf("file permissions not preserved: %s", put["projectFilePermissions"])
	}
}

func TestRoleClientDeleteIgnoresNotFound(t *testing.T) {
	status := http.StatusNotFound
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
	defer srv.Close()
	c := &roleClient{baseURL: srv.URL, token: "tok"}

	if err := c.delete(context.Background(), "r1"); err != nil {
		t.Errorf("404 should be tolerated: %v", err)
	}
	status = http.StatusInternalServerError
	if err := c.delete(context.Background(), "r1"); err == nil {
		t.Error("500 should fail")
	}
}

func TestModelFromAPINormalisesEmptyCollections(t *testing.T) {
	item := &roleAPIItem{ID: "r1", Name: "n", ProjectPermissions: []string{"documents:view-content"}}

	got := modelFromAPI(roleModel{}, item)
	if !got.IdpGroups.IsNull() || !got.ProjectIDs.IsNull() || !got.Description.IsNull() {
		t.Errorf("omitted attributes should stay null: %+v", got)
	}

	prior := roleModel{IdpGroups: stringSet(nil), ProjectIDs: stringSet(nil)}
	got = modelFromAPI(prior, item)
	if got.IdpGroups.IsNull() || got.ProjectIDs.IsNull() {
		t.Errorf("explicit empty sets should stay empty: %+v", got)
	}
}

func TestModelFromAPIProjectIDsFromGroups(t *testing.T) {
	item := &roleAPIItem{ID: "r1", Name: "n", IdpGroups: []string{"g"}}
	item.Groups = []roleAPIGroup{{"p2"}, {"p1"}, {"p2"}}

	got := modelFromAPI(roleModel{}, item)
	if !got.ProjectIDs.Equal(stringSet([]string{"p1", "p2"})) {
		t.Errorf("unexpected project ids: %s", got.ProjectIDs)
	}

	item.AllProjects = true
	prior := roleModel{ProjectIDs: stringSet([]string{"x"})}
	if got := modelFromAPI(prior, item); !got.ProjectIDs.Equal(prior.ProjectIDs) {
		t.Errorf("all_projects should keep prior project_ids, got %s", got.ProjectIDs)
	}
}
