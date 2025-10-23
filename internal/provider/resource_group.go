package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// -----------------------------------------------------------------------------
// Resource & Model
// -----------------------------------------------------------------------------

type groupResource struct{ cfg *Config }

type groupModel struct {
	ID          types.String   `tfsdk:"id"`
	ProjectID   types.String   `tfsdk:"project_id"`
	Name        types.String   `tfsdk:"name"`
	Description types.String   `tfsdk:"description"`
	Permissions []types.String `tfsdk:"permissions"`
}

func NewGroupResource() resource.Resource { return &groupResource{} }

func (r *groupResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "gorules_group"
}

func (r *groupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.cfg = req.ProviderData.(*Config)
}

func (r *groupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		MarkdownDescription: "Manages groups for a project in GoRules.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Group ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": rschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Parent project ID.",
			},
			"name": rschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Group name.",
			},
			"description": rschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Group description.",
			},
			"permissions": rschema.ListAttribute{
				ElementType:         types.StringType,
				Required:            true,
				MarkdownDescription: "Group permissions.",
			},
		},
	}
}

// -----------------------------------------------------------------------------

type groupCreateUpdateRequest struct {
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Permissions []string `json:"permissions"`
}

// normalize and sort permissions for comparison without noise
func (r *groupResource) normalizePerms(in []string) []string {
	cp := make([]string, len(in))
	copy(cp, in)
	sort.Strings(cp)
	return cp
}

// Paginated list with real schema {results, paginate}
func (r *groupResource) listAllGroups(ctx context.Context, projectID string) ([]groupItem, int, []byte, error) {
	perPage := 200
	page := 1
	collected := make([]groupItem, 0, perPage)

	for {
		url := fmt.Sprintf("%s/api/projects/%s/groups?perPage=%d&page=%d", r.cfg.BaseURL, projectID, perPage, page)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+r.cfg.Token)

		res, err := clientNoRedirect(r.cfg.HTTP).Do(req)
		if err != nil {
			return nil, 0, nil, err
		}
		raw, _ := io.ReadAll(res.Body)
		res.Body.Close()

		if res.StatusCode >= 300 {
			return nil, res.StatusCode, raw, fmt.Errorf("status=%d", res.StatusCode)
		}

		var gl groupListResponse // definido en http_utils.go
		if err := json.Unmarshal(raw, &gl); err != nil {
			return nil, res.StatusCode, raw, err
		}

		// permissions can come null → leave as []
		for i := range gl.Results {
			if gl.Results[i].Permissions == nil {
				gl.Results[i].Permissions = []string{}
			}
		}

		collected = append(collected, gl.Results...)

		// end of pagination
		if gl.Paginate.Total == 0 || gl.Paginate.PageSize == 0 {
			return collected, res.StatusCode, raw, nil
		}
		if len(collected) >= gl.Paginate.Total {
			return collected, res.StatusCode, raw, nil
		}
		page++
	}
}

// -----------------------------------------------------------------------------
// Create
// -----------------------------------------------------------------------------

func (r *groupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.cfg == nil {
		resp.Diagnostics.AddError("provider not configured", "Missing base_url/token")
		return
	}

	var plan groupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var descPtr *string
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		d := plan.Description.ValueString()
		descPtr = &d
	}

	perms := make([]string, 0, len(plan.Permissions))
	for _, p := range plan.Permissions {
		if !p.IsNull() && !p.IsUnknown() && p.ValueString() != "" {
			perms = append(perms, p.ValueString())
		}
	}
	perms = r.normalizePerms(perms)

	body := groupCreateUpdateRequest{
		Name:        plan.Name.ValueString(),
		Description: descPtr,
		Permissions: perms,
	}

	b, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/api/projects/%s/groups", r.cfg.BaseURL, plan.ProjectID.ValueString())
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	httpReq.Header.Set("Authorization", "Bearer "+r.cfg.Token)
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := clientNoRedirect(r.cfg.HTTP).Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Group", err.Error())
		return
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		resp.Diagnostics.AddError("Create Group falló",
			fmt.Sprintf("status=%d body=%s", res.StatusCode, string(raw)))
		return
	}

	var created groupItem // definido en http_utils.go
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Error parsing Create Group response", err.Error())
		return
	}
	if created.Permissions == nil {
		created.Permissions = []string{}
	}
	created.Permissions = r.normalizePerms(created.Permissions)

	state := groupModel{
		ID:        types.StringValue(created.ID),
		ProjectID: types.StringValue(plan.ProjectID.ValueString()),
		Name:      types.StringValue(created.Name),
	}
	if created.Description != nil {
		state.Description = types.StringValue(*created.Description)
	} else if descPtr != nil {
		state.Description = types.StringValue(*descPtr)
	} else {
		state.Description = types.StringValue("")
	}
	state.Permissions = ToTFStringList(created.Permissions)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// -----------------------------------------------------------------------------
// Read
// -----------------------------------------------------------------------------

func (r *groupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.cfg == nil {
		resp.Diagnostics.AddError("provider not configured", "Missing base_url/token")
		return
	}
	var state groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	items, code, raw, err := r.listAllGroups(ctx, state.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Get Group falló", fmt.Sprintf("status=%d body=%s", code, string(raw)))
		return
	}

	var found *groupItem
	for i := range items {
		if items[i].ID == state.ID.ValueString() {
			found = &items[i]
			break
		}
	}
	if found == nil {
		resp.Diagnostics.AddWarning("Get Group no encontrado (se conserva estado)",
			"No se halló el group en la lista paginada; se conserva el estado para evitar recreación por error.")
		return
	}

	if found.Permissions == nil {
		found.Permissions = []string{}
	}
	found.Permissions = r.normalizePerms(found.Permissions)

	state.Name = types.StringValue(found.Name)
	if found.Description != nil {
		state.Description = types.StringValue(*found.Description)
	} else {
		state.Description = types.StringValue("")
	}
	state.Permissions = ToTFStringList(found.Permissions)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// -----------------------------------------------------------------------------
// Update
// -----------------------------------------------------------------------------

func (r *groupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.cfg == nil {
		resp.Diagnostics.AddError("provider not configured", "Missing base_url/token")
		return
	}

	var plan groupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var descPtr *string
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		d := plan.Description.ValueString()
		descPtr = &d
	}
	perms := make([]string, 0, len(plan.Permissions))
	for _, p := range plan.Permissions {
		if !p.IsNull() && !p.IsUnknown() && p.ValueString() != "" {
			perms = append(perms, p.ValueString())
		}
	}
	perms = r.normalizePerms(perms)

	body := groupCreateUpdateRequest{
		Name:        plan.Name.ValueString(),
		Description: descPtr,
		Permissions: perms,
	}

	b, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/api/projects/%s/groups/%s", r.cfg.BaseURL, plan.ProjectID.ValueString(), plan.ID.ValueString())
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(b))
	httpReq.Header.Set("Authorization", "Bearer "+r.cfg.Token)
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := clientNoRedirect(r.cfg.HTTP).Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating Group", err.Error())
		return
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		resp.Diagnostics.AddError("Update Group falló",
			fmt.Sprintf("status=%d body=%s", res.StatusCode, string(raw)))
		return
	}

	var updated groupItem
	if err := json.Unmarshal(raw, &updated); err != nil {
		resp.Diagnostics.AddError("Error parsing Update Group response", err.Error())
		return
	}
	if updated.Permissions == nil {
		updated.Permissions = []string{}
	}
	updated.Permissions = r.normalizePerms(updated.Permissions)

	state := groupModel{
		ID:        types.StringValue(updated.ID),
		ProjectID: types.StringValue(plan.ProjectID.ValueString()),
		Name:      types.StringValue(updated.Name),
	}
	if updated.Description != nil {
		state.Description = types.StringValue(*updated.Description)
	} else if descPtr != nil {
		state.Description = types.StringValue(*descPtr)
	} else {
		state.Description = types.StringValue("")
	}
	state.Permissions = ToTFStringList(updated.Permissions)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// -----------------------------------------------------------------------------
// Delete
// -----------------------------------------------------------------------------

func (r *groupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.cfg == nil {
		resp.Diagnostics.AddError("provider not configured", "Missing base_url/token")
		return
	}
	var state groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/projects/%s/groups/%s", r.cfg.BaseURL, state.ProjectID.ValueString(), state.ID.ValueString())
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	httpReq.Header.Set("Authorization", "Bearer "+r.cfg.Token)

	res, err := clientNoRedirect(r.cfg.HTTP).Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting Group", err.Error())
		return
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 && res.StatusCode != http.StatusNotFound {
		raw, _ := io.ReadAll(res.Body)
		resp.Diagnostics.AddWarning("Delete Group no confirmado",
			fmt.Sprintf("status=%d body=%s", res.StatusCode, string(raw)))
	}
	resp.State.RemoveResource(ctx)
}
