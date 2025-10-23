// internal/provider/resource_environment.go
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// -----------------------------------------------------------------------------
// Resource & Model
// -----------------------------------------------------------------------------

type environmentResource struct{ cfg *Config }

type environmentModel struct {
	ID             types.String   `tfsdk:"id"`
	ProjectID      types.String   `tfsdk:"project_id"`
	Name           types.String   `tfsdk:"name"`
	Key            types.String   `tfsdk:"key"`
	Type           types.String   `tfsdk:"type"`
	ApprovalMode   types.String   `tfsdk:"approval_mode"`
	ApprovalGroups []types.String `tfsdk:"approval_groups"` // Group NAMES (not IDs)
}

func NewEnvironmentResource() resource.Resource { return &environmentResource{} }

func (r *environmentResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "gorules_environment"
}

func (r *environmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.cfg = req.ProviderData.(*Config)
}

func (r *environmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		MarkdownDescription: "Manages environments for a project in GoRules.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Environment ID.",
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
				MarkdownDescription: "Environment name.",
			},
			"key": rschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Environment key (if not provided, uses name).",
			},
			"type": rschema.StringAttribute{
				Required:            true, // enum: brms | deployment
				MarkdownDescription: "Environment type (brms|deployment).",
			},
			"approval_mode": rschema.StringAttribute{
				Optional:            true, // none | require_one_per_team | none_create_request | require_any
				MarkdownDescription: "Environment approval mode.",
			},
			"approval_groups": rschema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true, // allows default to []
				MarkdownDescription: "List of group NAMES that approve.",
			},
		},
	}
}

// -----------------------------------------------------------------------------
// Local types for parsing API (LIST returns ARRAY)
// -----------------------------------------------------------------------------

// Structure to handle approval groups that can come as objects or strings
type approvalGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type envItem struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Type              string          `json:"type"`
	Key               string          `json:"key"`
	ProjectID         string          `json:"projectId"`
	ApprovalMode      *string         `json:"approvalMode,omitempty"`
	ApprovalGroups    []string        `json:"-"`                        // Calculated field after parsing
	RawApprovalGroups json.RawMessage `json:"approvalGroups,omitempty"` // For dynamic parsing
}

// UnmarshalJSON implements custom parsing to handle approvalGroups dynamically
func (e *envItem) UnmarshalJSON(data []byte) error {
	// Define an alias to avoid infinite recursion
	type Alias envItem
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Process approvalGroups dynamically
	if e.RawApprovalGroups != nil {
		// Try first as string array
		var stringArray []string
		if err := json.Unmarshal(e.RawApprovalGroups, &stringArray); err == nil {
			e.ApprovalGroups = stringArray
		} else {
			// Try as object array
			var objectArray []approvalGroup
			if err := json.Unmarshal(e.RawApprovalGroups, &objectArray); err == nil {
				e.ApprovalGroups = make([]string, len(objectArray))
				for i, obj := range objectArray {
					e.ApprovalGroups[i] = obj.ID
				}
			} else {
				// If unable to parse, initialize empty array
				e.ApprovalGroups = []string{}
			}
		}
	} else {
		e.ApprovalGroups = []string{}
	}

	return nil
}

// Payload para POST/PUT
type envCreateUpdateRequest struct {
	Name           string   `json:"name"`
	Key            *string  `json:"key,omitempty"`
	Type           string   `json:"type"`
	ApprovalMode   *string  `json:"approvalMode,omitempty"`
	ApprovalGroups []string `json:"approvalGroups,omitempty"` // IDs
}

// -----------------------------------------------------------------------------
// Helpers de API (LIST + find by ID dentro del listado)
// -----------------------------------------------------------------------------

func (r *environmentResource) listEnvironments(ctx context.Context, projectID string) ([]envItem, int, []byte, error) {
	url := fmt.Sprintf("%s/api/projects/%s/environments", r.cfg.BaseURL, projectID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+r.cfg.Token)

	res, err := clientNoRedirect(r.cfg.HTTP).Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, res.StatusCode, raw, fmt.Errorf("status=%d", res.StatusCode)
	}

	var arr []envItem // la API entrega un ARRAY
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, res.StatusCode, raw, err
	}
	return arr, res.StatusCode, raw, nil
}

func (r *environmentResource) findEnvironmentByID(ctx context.Context, projectID, envID string) (*envItem, int, []byte, error) {
	arr, code, raw, err := r.listEnvironments(ctx, projectID)
	if err != nil {
		return nil, code, raw, err
	}
	for _, it := range arr {
		if it.ID == envID {
			// normalizamos arrays para consistencia
			sort.Strings(it.ApprovalGroups) // son IDs aquí
			return &it, code, raw, nil
		}
	}
	// No hay GET by ID; simulamos 404 si no aparece
	return nil, http.StatusNotFound, raw, fmt.Errorf("environment not found in listing")
}

// -----------------------------------------------------------------------------
// Create
// -----------------------------------------------------------------------------

func (r *environmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.cfg == nil {
		resp.Diagnostics.AddError("provider not configured", "Missing base_url/token")
		return
	}

	var plan environmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// default key = name
	var keyPtr *string
	if !plan.Key.IsNull() && !plan.Key.IsUnknown() && plan.Key.ValueString() != "" {
		k := plan.Key.ValueString()
		keyPtr = &k
	} else {
		k := plan.Name.ValueString()
		keyPtr = &k
	}

	// optional approval_mode
	var approvalModePtr *string
	if !plan.ApprovalMode.IsNull() && !plan.ApprovalMode.IsUnknown() && plan.ApprovalMode.ValueString() != "" {
		am := plan.ApprovalMode.ValueString()
		approvalModePtr = &am
	}

	// NAMES → IDs
	names := make([]string, 0, len(plan.ApprovalGroups))
	for _, s := range plan.ApprovalGroups {
		if !s.IsNull() && !s.IsUnknown() && s.ValueString() != "" {
			names = append(names, s.ValueString())
		}
	}
	sort.Strings(names)
	groupIDs, err := ResolveGroupIDsByName(ctx, r.cfg, plan.ProjectID.ValueString(), names)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving approval_groups (names→IDs)", err.Error())
		return
	}
	sort.Strings(groupIDs)

	body := envCreateUpdateRequest{
		Name:           plan.Name.ValueString(),
		Key:            keyPtr,
		Type:           plan.Type.ValueString(),
		ApprovalMode:   approvalModePtr,
		ApprovalGroups: groupIDs, // IDs to API
	}

	b, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/api/projects/%s/environments", r.cfg.BaseURL, plan.ProjectID.ValueString())
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	httpReq.Header.Set("Authorization", "Bearer "+r.cfg.Token)
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := clientNoRedirect(r.cfg.HTTP).Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Environment", err.Error())
		return
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		resp.Diagnostics.AddError("Create Environment falló",
			fmt.Sprintf("status=%d body=%s", res.StatusCode, string(raw)))
		return
	}

	var created envItem
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Error parsing Create Environment response", err.Error())
		return
	}

	// IDs → NOMBRES para guardar en state igual que el plan
	namesBack, err := ResolveGroupNamesByID(ctx, r.cfg, plan.ProjectID.ValueString(), created.ApprovalGroups)
	if err != nil {
		resp.Diagnostics.AddWarning("Could not resolve group names from IDs", err.Error())
		// en caso de error, usamos lo del plan
		namesBack = names
	}
	sort.Strings(namesBack)

	state := environmentModel{
		ID:             types.StringValue(created.ID),
		ProjectID:      types.StringValue(plan.ProjectID.ValueString()),
		Name:           types.StringValue(created.Name),
		Key:            types.StringValue(created.Key),
		Type:           types.StringValue(created.Type),
		ApprovalGroups: ToTFStringList(namesBack), // NOMBRES en state
	}
	if created.ApprovalMode != nil {
		state.ApprovalMode = types.StringValue(*created.ApprovalMode)
	} else if approvalModePtr != nil {
		state.ApprovalMode = types.StringValue(*approvalModePtr)
	} else {
		state.ApprovalMode = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// -----------------------------------------------------------------------------
// Read
// -----------------------------------------------------------------------------

func (r *environmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.cfg == nil {
		resp.Diagnostics.AddError("provider not configured", "Missing base_url/token")
		return
	}
	var state environmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	found, code, raw, err := r.findEnvironmentByID(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if code == http.StatusNotFound {
			resp.Diagnostics.AddWarning("Get Environment not found (se conserva estado)",
				"No se halló el environment en la lista; se conserva el estado para evitar recreación por error.")
			return
		}
		resp.Diagnostics.AddError("Get Environment falló",
			fmt.Sprintf("status=%d\nbody=%s", code, string(raw)))
		return
	}

	// IDs → NOMBRES para state
	names, err := ResolveGroupNamesByID(ctx, r.cfg, state.ProjectID.ValueString(), found.ApprovalGroups)
	if err != nil {
		// mantenemos lo que ya había en state
		resp.Diagnostics.AddWarning("Could not resolve group names from IDs",
			"Se conserva el valor actual en estado.")
		names = make([]string, 0, len(state.ApprovalGroups))
		for _, s := range state.ApprovalGroups {
			if !s.IsNull() && !s.IsUnknown() && s.ValueString() != "" {
				names = append(names, s.ValueString())
			}
		}
	}
	sort.Strings(names)

	state.Name = types.StringValue(found.Name)
	state.Key = types.StringValue(found.Key)
	state.Type = types.StringValue(found.Type)
	if found.ApprovalMode != nil {
		state.ApprovalMode = types.StringValue(*found.ApprovalMode)
	} else {
		state.ApprovalMode = types.StringNull()
	}
	state.ApprovalGroups = ToTFStringList(names)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// -----------------------------------------------------------------------------
// Update
// -----------------------------------------------------------------------------

func (r *environmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.cfg == nil {
		resp.Diagnostics.AddError("provider not configured", "Missing base_url/token")
		return
	}

	var plan environmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// default key = name
	var keyPtr *string
	if !plan.Key.IsNull() && !plan.Key.IsUnknown() && plan.Key.ValueString() != "" {
		k := plan.Key.ValueString()
		keyPtr = &k
	} else {
		k := plan.Name.ValueString()
		keyPtr = &k
	}

	// optional approval_mode
	var approvalModePtr *string
	if !plan.ApprovalMode.IsNull() && !plan.ApprovalMode.IsUnknown() && plan.ApprovalMode.ValueString() != "" {
		am := plan.ApprovalMode.ValueString()
		approvalModePtr = &am
	}

	// NAMES → IDs
	names := make([]string, 0, len(plan.ApprovalGroups))
	for _, s := range plan.ApprovalGroups {
		if !s.IsNull() && !s.IsUnknown() && s.ValueString() != "" {
			names = append(names, s.ValueString())
		}
	}
	sort.Strings(names)
	groupIDs, err := ResolveGroupIDsByName(ctx, r.cfg, plan.ProjectID.ValueString(), names)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving approval_groups (names→IDs)", err.Error())
		return
	}
	sort.Strings(groupIDs)

	body := envCreateUpdateRequest{
		Name:           plan.Name.ValueString(),
		Key:            keyPtr,
		Type:           plan.Type.ValueString(),
		ApprovalMode:   approvalModePtr,
		ApprovalGroups: groupIDs,
	}

	b, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/api/projects/%s/environments/%s", r.cfg.BaseURL, plan.ProjectID.ValueString(), plan.ID.ValueString())
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(b))
	httpReq.Header.Set("Authorization", "Bearer "+r.cfg.Token)
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := clientNoRedirect(r.cfg.HTTP).Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating Environment", err.Error())
		return
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		resp.Diagnostics.AddError("Update Environment falló",
			fmt.Sprintf("status=%d body=%s", res.StatusCode, string(raw)))
		return
	}

	var updated envItem
	if err := json.Unmarshal(raw, &updated); err != nil {
		resp.Diagnostics.AddError("Error parsing Update Environment response", err.Error())
		return
	}

	// IDs → NOMBRES
	namesBack, err := ResolveGroupNamesByID(ctx, r.cfg, plan.ProjectID.ValueString(), updated.ApprovalGroups)
	if err != nil {
		resp.Diagnostics.AddWarning("Could not resolve group names from IDs", err.Error())
		namesBack = names
	}
	sort.Strings(namesBack)

	state := environmentModel{
		ID:             types.StringValue(updated.ID),
		ProjectID:      types.StringValue(plan.ProjectID.ValueString()),
		Name:           types.StringValue(updated.Name),
		Key:            types.StringValue(updated.Key),
		Type:           types.StringValue(updated.Type),
		ApprovalGroups: ToTFStringList(namesBack),
	}
	if updated.ApprovalMode != nil {
		state.ApprovalMode = types.StringValue(*updated.ApprovalMode)
	} else {
		state.ApprovalMode = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// -----------------------------------------------------------------------------
// Delete (con reintentos para 5xx)
// -----------------------------------------------------------------------------

func (r *environmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.cfg == nil {
		resp.Diagnostics.AddError("provider not configured", "Missing base_url/token")
		return
	}
	var state environmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/projects/%s/environments/%s", r.cfg.BaseURL, state.ProjectID.ValueString(), state.ID.ValueString())

	var lastStatus int
	var lastBody []byte
	for i := 0; i < 3; i++ {
		httpReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		httpReq.Header.Set("Authorization", "Bearer "+r.cfg.Token)

		res, err := clientNoRedirect(r.cfg.HTTP).Do(httpReq)
		if err != nil {
			lastStatus = 0
			lastBody = []byte(err.Error())
		} else {
			func() {
				defer res.Body.Close()
				lastStatus = res.StatusCode
				lastBody, _ = io.ReadAll(res.Body)
			}()
		}
		if lastStatus == http.StatusNotFound || (lastStatus >= 200 && lastStatus < 300) {
			resp.State.RemoveResource(ctx)
			return
		}
		time.Sleep(400 * time.Millisecond) // backoff simple
	}

	resp.Diagnostics.AddError("Delete Environment no confirmado tras reintentos",
		fmt.Sprintf("último status=%d body=%s", lastStatus, string(lastBody)))
}
