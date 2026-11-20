package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// -----------------------------------------------------------------------------
// Resource & Model
// -----------------------------------------------------------------------------

type roleResource struct{ cfg *Config }

type roleModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	IdpGroups   types.Set    `tfsdk:"idp_groups"`
	AllProjects types.Bool   `tfsdk:"all_projects"`
	ProjectIDs  types.Set    `tfsdk:"project_ids"`
	Permissions types.Set    `tfsdk:"permissions"`
}

var rolePermissions = []string{
	"owner",
	"documents",
	"releases",
	"releases:manage",
	"releases:deploy",
	"releases:delete",
	"integrations:manage",
	"documents:full",
	"documents:view-content",
	"documents:edit-content",
	"documents:edit-view",
	"branches",
	"branches:create",
	"branches:merge",
	"branches:delete",
	"project:manage",
	"environments",
	"environments:manage",
	"environments:delete",
}

func NewRoleResource() resource.Resource { return &roleResource{} }

func (r *roleResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "gorules_role"
}

func (r *roleResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.cfg = req.ProviderData.(*Config)
}

func (r *roleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		MarkdownDescription: "Manages an organisation-level role in GoRules. With per-project role mapping enabled, " +
			"a role grants project permissions to the members of the mapped identity provider groups.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Role ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": rschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Role name.",
			},
			"description": rschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Role description.",
			},
			"idp_groups": rschema.SetAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Identity provider group names mapped to this role.",
			},
			"all_projects": rschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether the role applies to all projects. When `true`, `project_ids` is ignored by the API.",
			},
			"project_ids": rschema.SetAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "IDs of the projects the role applies to.",
			},
			"permissions": rschema.SetAttribute{
				ElementType:         types.StringType,
				Required:            true,
				MarkdownDescription: "Project permissions granted by the role.",
				Validators:          []validator.Set{rolePermissionsValidator{}},
			},
		},
	}
}

// -----------------------------------------------------------------------------
// Permission validator
// -----------------------------------------------------------------------------

type rolePermissionsValidator struct{}

func (rolePermissionsValidator) Description(context.Context) string {
	return "each value must be a valid GoRules project permission"
}

func (v rolePermissionsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (rolePermissionsValidator) ValidateSet(ctx context.Context, req validator.SetRequest, resp *validator.SetResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	valid := make(map[string]struct{}, len(rolePermissions))
	for _, p := range rolePermissions {
		valid[p] = struct{}{}
	}
	var values []string
	resp.Diagnostics.Append(req.ConfigValue.ElementsAs(ctx, &values, false)...)
	for _, v := range values {
		if _, ok := valid[v]; !ok {
			resp.Diagnostics.AddAttributeError(req.Path, "Invalid permission",
				fmt.Sprintf("%q is not a valid project permission; valid values: %v", v, rolePermissions))
		}
	}
}

// -----------------------------------------------------------------------------
// API client
// -----------------------------------------------------------------------------

var errRoleNotFound = errors.New("role not found")

type roleClient struct {
	baseURL string
	token   string
	http    *http.Client
}

type roleAPIGroup struct {
	ProjectID string `json:"projectId"`
}

type roleAPIItem struct {
	ID                     string          `json:"id"`
	Name                   string          `json:"name"`
	Description            *string         `json:"description"`
	IdpGroups              []string        `json:"idpGroups"`
	AllProjects            bool            `json:"allProjects"`
	ProjectPermissions     []string        `json:"projectPermissions"`
	ProjectFilePermissions json.RawMessage `json:"projectFilePermissions"`
	Groups                 []roleAPIGroup  `json:"groups"`
	Users                  []struct {
		ID string `json:"id"`
	} `json:"users"`
}

type roleListResponse struct {
	Results  []roleAPIItem `json:"results"`
	Paginate struct {
		Total int `json:"total"`
	} `json:"paginate"`
}

type roleWriteBody struct {
	Name                   string          `json:"name"`
	Description            string          `json:"description"`
	IdpGroups              []string        `json:"idpGroups"`
	ProjectPermissions     []string        `json:"projectPermissions"`
	ProjectFilePermissions json.RawMessage `json:"projectFilePermissions,omitempty"`
	AllProjects            bool            `json:"allProjects"`
	Users                  *[]string       `json:"users,omitempty"`
	Projects               []string        `json:"projects"`
}

const roleListPageSize = 100

func newRoleClient(cfg *Config) *roleClient {
	return &roleClient{baseURL: cfg.BaseURL, token: cfg.Token, http: cfg.HTTP}
}

func (c *roleClient) do(ctx context.Context, method, urlPath string, body any) (int, []byte, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+urlPath, rdr)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := clientNoRedirect(c.http).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	return res.StatusCode, raw, err
}

func (c *roleClient) create(ctx context.Context, body roleWriteBody) (*roleAPIItem, error) {
	status, raw, err := c.do(ctx, http.MethodPost, "/api/roles", body)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, fmt.Errorf("status=%d body=%s", status, string(raw))
	}
	var item roleAPIItem
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &item, nil
}

// get reads a role from the paginated list; the API has no get-by-id endpoint.
func (c *roleClient) get(ctx context.Context, id string) (*roleAPIItem, error) {
	collected := 0
	for page := 1; ; page++ {
		status, raw, err := c.do(ctx, http.MethodGet,
			fmt.Sprintf("/api/roles?perPage=%d&page=%d", roleListPageSize, page), nil)
		if err != nil {
			return nil, err
		}
		if status >= 300 {
			return nil, fmt.Errorf("status=%d body=%s", status, string(raw))
		}
		var list roleListResponse
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		for i := range list.Results {
			if list.Results[i].ID == id {
				return &list.Results[i], nil
			}
		}
		collected += len(list.Results)
		if len(list.Results) == 0 || collected >= list.Paginate.Total {
			return nil, errRoleNotFound
		}
	}
}

// update replaces the role while carrying over the users and file permissions
// currently stored in the API, which this resource does not manage.
func (c *roleClient) update(ctx context.Context, id string, body roleWriteBody) (*roleAPIItem, error) {
	current, err := c.get(ctx, id)
	if err != nil {
		return nil, err
	}

	users := make([]string, 0, len(current.Users))
	for _, u := range current.Users {
		users = append(users, u.ID)
	}
	body.Users = &users
	body.ProjectFilePermissions = current.ProjectFilePermissions
	if len(body.ProjectFilePermissions) == 0 || string(body.ProjectFilePermissions) == "null" {
		body.ProjectFilePermissions = json.RawMessage("[]")
	}

	status, raw, err := c.do(ctx, http.MethodPut, "/api/roles/"+id, body)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, errRoleNotFound
	}
	if status >= 300 {
		return nil, fmt.Errorf("status=%d body=%s", status, string(raw))
	}
	var item roleAPIItem
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &item, nil
}

// delete treats an already-missing role as deleted.
func (c *roleClient) delete(ctx context.Context, id string) error {
	status, raw, err := c.do(ctx, http.MethodDelete, "/api/roles/"+id, nil)
	if err != nil {
		return err
	}
	if status >= 300 && status != http.StatusNotFound {
		return fmt.Errorf("status=%d body=%s", status, string(raw))
	}
	return nil
}

// -----------------------------------------------------------------------------
// Model <-> API conversion
// -----------------------------------------------------------------------------

func setToSortedStrings(ctx context.Context, s types.Set) ([]string, diag.Diagnostics) {
	out := []string{}
	if s.IsNull() || s.IsUnknown() {
		return out, nil
	}
	diags := s.ElementsAs(ctx, &out, false)
	sort.Strings(out)
	return out, diags
}

func (m roleModel) toWriteBody(ctx context.Context) (roleWriteBody, diag.Diagnostics) {
	var diags diag.Diagnostics
	idp, d := setToSortedStrings(ctx, m.IdpGroups)
	diags.Append(d...)
	projects, d := setToSortedStrings(ctx, m.ProjectIDs)
	diags.Append(d...)
	perms, d := setToSortedStrings(ctx, m.Permissions)
	diags.Append(d...)

	allProjects := m.AllProjects.ValueBool()
	if allProjects {
		projects = []string{}
	}
	return roleWriteBody{
		Name:               m.Name.ValueString(),
		Description:        m.Description.ValueString(),
		IdpGroups:          idp,
		ProjectPermissions: perms,
		AllProjects:        allProjects,
		Projects:           projects,
	}, diags
}

// optionalStringSet keeps a null prior value null when the API returns an empty
// collection, so configs that omit the attribute do not drift.
func optionalStringSet(prior types.Set, values []string) types.Set {
	if len(values) == 0 && prior.IsNull() {
		return types.SetNull(types.StringType)
	}
	return stringSet(values)
}

func stringSet(values []string) types.Set {
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	return types.SetValueMust(types.StringType, elems)
}

func projectIDsFromGroups(item *roleAPIItem) []string {
	seen := map[string]struct{}{}
	ids := []string{}
	for _, g := range item.Groups {
		if g.ProjectID == "" {
			continue
		}
		if _, ok := seen[g.ProjectID]; !ok {
			seen[g.ProjectID] = struct{}{}
			ids = append(ids, g.ProjectID)
		}
	}
	sort.Strings(ids)
	return ids
}

// modelFromAPI builds the state for a role read from the API. Project IDs come
// from the role's groups, which the API creates one per assigned project.
func modelFromAPI(prior roleModel, item *roleAPIItem) roleModel {
	desc := types.StringNull()
	if item.Description != nil && *item.Description != "" {
		desc = types.StringValue(*item.Description)
	} else if !prior.Description.IsNull() {
		desc = types.StringValue("")
	}

	projectIDs := prior.ProjectIDs
	if !item.AllProjects {
		projectIDs = optionalStringSet(prior.ProjectIDs, projectIDsFromGroups(item))
	}
	if projectIDs.IsUnknown() {
		projectIDs = types.SetNull(types.StringType)
	}

	return roleModel{
		ID:          types.StringValue(item.ID),
		Name:        types.StringValue(item.Name),
		Description: desc,
		IdpGroups:   optionalStringSet(prior.IdpGroups, item.IdpGroups),
		AllProjects: types.BoolValue(item.AllProjects),
		ProjectIDs:  projectIDs,
		Permissions: stringSet(item.ProjectPermissions),
	}
}

// -----------------------------------------------------------------------------
// CRUD
// -----------------------------------------------------------------------------

func (r *roleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.cfg == nil {
		resp.Diagnostics.AddError("provider not configured", "Missing base_url/token")
		return
	}
	var plan roleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, diags := plan.toWriteBody(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := newRoleClient(r.cfg).create(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Role", err.Error())
		return
	}

	plan.ID = types.StringValue(created.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *roleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.cfg == nil {
		resp.Diagnostics.AddError("provider not configured", "Missing base_url/token")
		return
	}
	var state roleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	item, err := newRoleClient(r.cfg).get(ctx, state.ID.ValueString())
	if errors.Is(err, errRoleNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Role", err.Error())
		return
	}

	newState := modelFromAPI(state, item)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *roleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.cfg == nil {
		resp.Diagnostics.AddError("provider not configured", "Missing base_url/token")
		return
	}
	var plan, state roleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, diags := plan.toWriteBody(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := newRoleClient(r.cfg).update(ctx, state.ID.ValueString(), body); err != nil {
		resp.Diagnostics.AddError("Error updating Role", err.Error())
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *roleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.cfg == nil {
		resp.Diagnostics.AddError("provider not configured", "Missing base_url/token")
		return
	}
	var state roleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := newRoleClient(r.cfg).delete(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting Role", err.Error())
	}
}

func (r *roleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
