package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// -----------------------------------------------------------------------------
// HTTP client without redirects (maintains Authorization header)
// -----------------------------------------------------------------------------
func clientNoRedirect(base *http.Client) *http.Client {
	if base == nil {
		return &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	c := *base
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	return &c
}

// -----------------------------------------------------------------------------
// Various utilities
// -----------------------------------------------------------------------------

// slugify converts a string to kebab-case (useful for resource names)
var reNotAllowed = regexp.MustCompile(`[^a-z0-9-]`)
var reDashCollapse = regexp.MustCompile(`-+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = reNotAllowed.ReplaceAllString(s, "-")
	s = reDashCollapse.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// -----------------------------------------------------------------------------
// Shared Terraform helpers (for all resources)
// -----------------------------------------------------------------------------

// ToTFStringList converts []string to []types.String
func ToTFStringList(xs []string) []types.String {
	out := make([]types.String, 0, len(xs))
	for _, v := range xs {
		out = append(out, types.StringValue(v))
	}
	return out
}

// EmptyTFStringList returns an empty list of TF strings (not null)
func EmptyTFStringList() []types.String {
	return []types.String{}
}

// ToTFStringListSorted converts and SORTS []string -> []types.String
func ToTFStringListSorted(xs []string) []types.String {
	cp := make([]string, len(xs))
	copy(cp, xs)
	sort.Strings(cp)
	return ToTFStringList(cp)
}

// -----------------------------------------------------------------------------
// Helpers for resolving Groups (ID <-> Name)
// -----------------------------------------------------------------------------

type groupItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Permissions []string `json:"permissions"` // can come null in JSON; json.Unmarshal leaves nil
	RoleID      *string  `json:"roleId,omitempty"`
}

type groupListResponse struct {
	Results  []groupItem `json:"results"`
	Paginate struct {
		PageSize int `json:"pageSize"`
		Current  int `json:"current"`
		Total    int `json:"total"`
		From     int `json:"from"`
		To       int `json:"to"`
	} `json:"paginate"`
}

// Returns group IDs from their names
func ResolveGroupIDsByName(ctx context.Context, cfg *Config, projectID string, names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	url := fmt.Sprintf("%s/api/projects/%s/groups?perPage=500", cfg.BaseURL, projectID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	res, err := clientNoRedirect(cfg.HTTP).Do(req)
	if err != nil {
		return nil, fmt.Errorf("error listing groups: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		raw, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("status=%d body=%s", res.StatusCode, string(raw))
	}

	var gl groupListResponse
	if err := json.NewDecoder(res.Body).Decode(&gl); err != nil {
		return nil, fmt.Errorf("error parsing groups: %w", err)
	}

	byName := map[string]string{}
	for _, it := range gl.Results {
		byName[it.Name] = it.ID
	}

	ids := make([]string, 0, len(names))
	for _, n := range names {
		if id, ok := byName[n]; ok {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// Returns group names from their IDs
func ResolveGroupNamesByID(ctx context.Context, cfg *Config, projectID string, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return []string{}, nil
	}
	url := fmt.Sprintf("%s/api/projects/%s/groups?perPage=500", cfg.BaseURL, projectID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	res, err := clientNoRedirect(cfg.HTTP).Do(req)
	if err != nil {
		return nil, fmt.Errorf("error listing groups: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		raw, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("status=%d body=%s", res.StatusCode, string(raw))
	}

	var gl groupListResponse
	if err := json.NewDecoder(res.Body).Decode(&gl); err != nil {
		return nil, fmt.Errorf("error parsing groups: %w", err)
	}

	byID := map[string]string{}
	for _, it := range gl.Results {
		byID[it.ID] = it.Name
	}

	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if n, ok := byID[id]; ok {
			names = append(names, n)
		}
	}
	return names, nil
}
