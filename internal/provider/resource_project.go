package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// -----------------------------------------------------------------------------
// Resource: gorules_project
// -----------------------------------------------------------------------------

type projectResource struct{ cfg *Config }

type projectModel struct {
	ID             types.String `tfsdk:"id"`               // UUID returned by the API
	Name           types.String `tfsdk:"name"`             // required
	Key            types.String `tfsdk:"key"`              // required: ^[a-z0-9]{2,}(-[a-z0-9]+)*$
	Protected      types.Bool   `tfsdk:"protected"`        // optional+computed
	CopyContentRef types.String `tfsdk:"copy_content_ref"` // optional: UUID to copy
}

// Payload for creating/updating
type createProjectRequest struct {
	Name           string `json:"name"`
	Key            string `json:"key"`
	Protected      *bool  `json:"protected,omitempty"`
	CopyContentRef string `json:"copyContentRef,omitempty"`
}

// Flat response
type projectFlat struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Key       string `json:"key"`
	Protected *bool  `json:"protected,omitempty"`
}

// Wrapped response: { "project": {...} }
type projectEnvelope struct {
	Project projectFlat `json:"project"`
}

// Flexible parser
func parseProjectJSON(raw []byte) (projectFlat, error) {
	var env projectEnvelope
	if err := json.Unmarshal(raw, &env); err == nil && (env.Project.ID != "" || env.Project.Name != "" || env.Project.Key != "") {
		return env.Project, nil
	}
	var pf projectFlat
	if err := json.Unmarshal(raw, &pf); err == nil && (pf.ID != "" || pf.Name != "" || pf.Key != "") {
		return pf, nil
	}
	return projectFlat{}, fmt.Errorf("could not parse project response")
}

// Helper: use server if provided; otherwise keep plan; if both empty → null
func firstNonEmptyStringTF(server string, plan types.String) types.String {
	if server != "" {
		return types.StringValue(server)
	}
	if !plan.IsNull() && !plan.IsUnknown() && plan.ValueString() != "" {
		return plan
	}
	return types.StringNull()
}

func NewProjectResource() resource.Resource { return &projectResource{} }

func (r *projectResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "gorules_project"
}

func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		MarkdownDescription: "Creates and manages projects in GoRules.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Project ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": rschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Project name.",
			},
			"key": rschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Unique key (regex: `^[a-z0-9]{2,}(-[a-z0-9]+)*$`).",
			},
			"protected": rschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "If true, marks the project as protected.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"copy_content_ref": rschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "UUID of project to copy (content clone).",
			},
		},
	}
}

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.cfg = req.ProviderData.(*Config)
}

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.cfg == nil {
		resp.Diagnostics.AddError("provider not configured", "Missing base_url/token")
		return
	}
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := createProjectRequest{
		Name: plan.Name.ValueString(),
		Key:  plan.Key.ValueString(),
	}
	if !plan.Protected.IsNull() && !plan.Protected.IsUnknown() {
		v := plan.Protected.ValueBool()
		body.Protected = &v
	}
	if !plan.CopyContentRef.IsNull() && !plan.CopyContentRef.IsUnknown() && plan.CopyContentRef.ValueString() != "" {
		body.CopyContentRef = plan.CopyContentRef.ValueString()
	}

	b, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/api/projects", r.cfg.BaseURL)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	httpReq.Header.Set("Authorization", "Bearer "+r.cfg.Token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	client := clientNoRedirect(r.cfg.HTTP)
	res, err := client.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating project", err.Error())
		return
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		resp.Diagnostics.AddWarning("Create Project returned redirect",
			fmt.Sprintf("status=%d location=%s", res.StatusCode, res.Header.Get("Location")))
	}
	if res.StatusCode >= 300 {
		resp.Diagnostics.AddAttributeError(path.Root("name"), "Create Project failed",
			fmt.Sprintf("status=%d body=%s", res.StatusCode, string(raw)))
		return
	}

	pf, _ := parseProjectJSON(raw)
	if pf.ID == "" {
		if loc := res.Header.Get("Location"); loc != "" {
			parts := strings.Split(strings.TrimRight(loc, "/"), "/")
			pf.ID = parts[len(parts)-1]
		}
	}

	// Hydrate with GET (tolerant)
	if pf.ID != "" {
		getURL := fmt.Sprintf("%s/api/projects/%s", r.cfg.BaseURL, pf.ID)
		getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
		getReq.Header.Set("Authorization", "Bearer "+r.cfg.Token)
		getReq.Header.Set("Accept", "application/json")

		if getRes, getErr := client.Do(getReq); getErr == nil && getRes != nil {
			defer getRes.Body.Close()
			if getRes.StatusCode < 300 {
				if hydrated, err := io.ReadAll(getRes.Body); err == nil {
					if pf2, e2 := parseProjectJSON(hydrated); e2 == nil {
						pf = pf2
					}
				}
			}
		}
	}

	nameTF := firstNonEmptyStringTF(pf.Name, plan.Name)
	keyTF := firstNonEmptyStringTF(pf.Key, plan.Key)

	var protectedTF types.Bool
	if pf.Protected != nil {
		protectedTF = types.BoolValue(*pf.Protected)
	} else if !plan.Protected.IsNull() && !plan.Protected.IsUnknown() {
		protectedTF = types.BoolValue(plan.Protected.ValueBool())
	} else {
		protectedTF = types.BoolNull()
	}

	state := projectModel{
		ID:             types.StringValue(pf.ID),
		Name:           nameTF,
		Key:            keyTF,
		Protected:      protectedTF,
		CopyContentRef: plan.CopyContentRef,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.cfg == nil {
		resp.Diagnostics.AddError("provider not configured", "Missing base_url/token")
		return
	}
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/projects/%s", r.cfg.BaseURL, state.ID.ValueString())
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	httpReq.Header.Set("Authorization", "Bearer "+r.cfg.Token)
	httpReq.Header.Set("Accept", "application/json")

	client := clientNoRedirect(r.cfg.HTTP)
	res, err := client.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Error reading project", err.Error())
		return
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		resp.Diagnostics.AddWarning("Get Project returned redirect",
			fmt.Sprintf("status=%d location=%s", res.StatusCode, res.Header.Get("Location")))
		return
	}
	if res.StatusCode >= 500 {
		raw, _ := io.ReadAll(res.Body)
		resp.Diagnostics.AddWarning("Get Project failed (5xx, preserving state)",
			fmt.Sprintf("status=%d body=%s", res.StatusCode, string(raw)))
		return
	}
	if res.StatusCode >= 400 {
		raw, _ := io.ReadAll(res.Body)
		resp.Diagnostics.AddWarning("Get Project returned 4xx (preserving state)",
			fmt.Sprintf("status=%d body=%s", res.StatusCode, string(raw)))
		return
	}

	raw, _ := io.ReadAll(res.Body)
	pf, err := parseProjectJSON(raw)
	if err != nil {
		resp.Diagnostics.AddWarning("Project parsing failed (preserving state)", err.Error())
		return
	}

	state.Name = firstNonEmptyStringTF(pf.Name, state.Name)
	state.Key = firstNonEmptyStringTF(pf.Key, state.Key)
	if pf.Protected != nil {
		state.Protected = types.BoolValue(*pf.Protected)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.cfg == nil {
		resp.Diagnostics.AddError("provider not configured", "Missing base_url/token")
		return
	}
	var plan, state projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var protectedVal *bool
	if !plan.Protected.IsNull() && !plan.Protected.IsUnknown() {
		v := plan.Protected.ValueBool()
		protectedVal = &v
	} else if !state.Protected.IsNull() && !state.Protected.IsUnknown() {
		v := state.Protected.ValueBool()
		protectedVal = &v
	} else {
		v := false
		protectedVal = &v
	}

	payload := createProjectRequest{
		Name:      plan.Name.ValueString(),
		Key:       plan.Key.ValueString(),
		Protected: protectedVal,
	}
	b, _ := json.Marshal(payload)

	url := fmt.Sprintf("%s/api/projects/%s", r.cfg.BaseURL, state.ID.ValueString())
	makeReq := func() *http.Request {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+r.cfg.Token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		return req
	}

	client := clientNoRedirect(r.cfg.HTTP)

	httpReq := makeReq()
	res, err := client.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating project", err.Error())
		return
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()

	if res.StatusCode >= 300 && res.StatusCode < 400 {
		resp.Diagnostics.AddError("Update Project returned redirect",
			fmt.Sprintf("status=%d location=%s", res.StatusCode, res.Header.Get("Location")))
		return
	}

	if res.StatusCode == http.StatusUnauthorized {
		// single soft retry
		httpReq = makeReq()
		res2, err2 := client.Do(httpReq)
		if err2 != nil {
			resp.Diagnostics.AddError("Error updating project (retry)", err2.Error())
			return
		}
		raw, _ = io.ReadAll(res2.Body)
		res2.Body.Close()
		res = res2
	}

	if res.StatusCode >= 300 {
		resp.Diagnostics.AddError("Update Project failed",
			fmt.Sprintf("status=%d body=%s", res.StatusCode, string(raw)))
		return
	}

	pf, _ := parseProjectJSON(raw)

	state.Name = firstNonEmptyStringTF(pf.Name, plan.Name)
	state.Key = firstNonEmptyStringTF(pf.Key, plan.Key)
	if pf.Protected != nil {
		state.Protected = types.BoolValue(*pf.Protected)
	} else if protectedVal != nil {
		state.Protected = types.BoolValue(*protectedVal)
	} else {
		state.Protected = types.BoolNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.cfg == nil {
		resp.Diagnostics.AddError("provider not configured", "Missing base_url/token")
		return
	}
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/projects/%s", r.cfg.BaseURL, state.ID.ValueString())
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	httpReq.Header.Set("Authorization", "Bearer "+r.cfg.Token)
	httpReq.Header.Set("Accept", "application/json")

	client := clientNoRedirect(r.cfg.HTTP)
	res, err := client.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting project", err.Error())
		return
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 && res.StatusCode != http.StatusNotFound {
		raw, _ := io.ReadAll(res.Body)
		resp.Diagnostics.AddWarning("Delete Project not confirmed",
			fmt.Sprintf("status=%d body=%s", res.StatusCode, string(raw)))
	}
	resp.State.RemoveResource(ctx)
}
