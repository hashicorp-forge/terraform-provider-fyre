// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp-forge/terraform-provider-fyre/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                     = &ResourceVM{}
	_ resource.ResourceWithImportState      = &ResourceVM{}
	_ resource.ResourceWithConfigValidators = &ResourceVM{}
)

// NewResourceVM creates a new VM resource.
func NewResourceVM() resource.Resource {
	return &ResourceVM{}
}

// ResourceVM defines the resource implementation.
type ResourceVM struct {
	client                *client.ClientWithResponses
	defaultSite           string
	defaultProductGroupID *int64
}

// VMModel describes the resource data model.
type VMModel struct {
	ID              types.String `tfsdk:"id"`
	VMID            types.String `tfsdk:"vm_id"`
	Site            types.String `tfsdk:"site"`
	QuotaType       types.String `tfsdk:"quota_type"`
	ProductGroupID  types.String `tfsdk:"product_group_id"`
	TimeToLive      types.String `tfsdk:"time_to_live"`
	Platform        types.String `tfsdk:"platform"`
	OS              types.String `tfsdk:"os"`
	CPU             types.Int64  `tfsdk:"cpu"`
	Memory          types.Int64  `tfsdk:"memory"`
	Hostname        types.String `tfsdk:"hostname"`
	Description     types.String `tfsdk:"description"`
	Expiration      types.String `tfsdk:"expiration"`
	ExpirationTime  types.String `tfsdk:"expiration_time"`
	PublicNetwork   types.String `tfsdk:"public_network"`
	DNS             types.String `tfsdk:"dns"`
	SSHKeys         types.List   `tfsdk:"ssh_keys"`
	Password        types.String `tfsdk:"password"`
	DisableDelete   types.String `tfsdk:"disable_delete"`
	AdditionalDisks types.List   `tfsdk:"additional_disks"`

	// Computed fields
	FQDN     types.String `tfsdk:"fqdn"`
	State    types.String `tfsdk:"state"`
	Created  types.String `tfsdk:"created"`
	Location types.String `tfsdk:"location"`
	IPs      types.List   `tfsdk:"ips"`
}

type quotaTypeValidator struct{}

// Metadata returns the resource type name.
func (r *ResourceVM) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm"
}

// Schema defines the schema for the resource.
func (r *ResourceVM) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Fyre VM resource with full lifecycle support. Note: The VM identifier can be vm_id, IP address, or FQDN for update and delete operations.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier for the VM (same as vm_id)",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"vm_id": schema.StringAttribute{
				MarkdownDescription: "The VM identifier (format: x-xxxxxxx)",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"site": schema.StringAttribute{
				MarkdownDescription: "Site location (svl or rtp). Defaults to 'svl' or inherits from provider configuration.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"quota_type": schema.StringAttribute{
				MarkdownDescription: "Type of quota to use (product_group or quick_burn). Defaults to product_group.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"product_group_id": schema.StringAttribute{
				MarkdownDescription: "Product group identifier. Required when quota_type is 'product_group'. Defaults to user's default product group.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"time_to_live": schema.StringAttribute{
				MarkdownDescription: "Time to live in hours. Required when quota_type is 'quick_burn'.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"platform": schema.StringAttribute{
				MarkdownDescription: "Platform type (x, pvm, or z). Defaults to x.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"os": schema.StringAttribute{
				MarkdownDescription: "Operating system (required). Use the fyre_vm_os_available data source to see available options.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cpu": schema.Int64Attribute{
				MarkdownDescription: "Number of CPUs. Defaults to 4. Can be updated after creation.",
				Optional:            true,
				Computed:            true,
			},
			"memory": schema.Int64Attribute{
				MarkdownDescription: "Memory in GB. Defaults to 8. Can be updated after creation.",
				Optional:            true,
				Computed:            true,
			},
			"hostname": schema.StringAttribute{
				MarkdownDescription: "Hostname for the VM. If not provided, will be auto-generated by the API. Can be updated after creation.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "VM description. Can be updated after creation.",
				Optional:            true,
			},
			"expiration": schema.StringAttribute{
				MarkdownDescription: "Expiration time in hours or 'X days X hours' format (max 90 days). Can be updated after creation.",
				Optional:            true,
			},
			"expiration_time": schema.StringAttribute{
				MarkdownDescription: "Absolute expiration timestamp returned by the API (computed, read-only)",
				Computed:            true,
			},
			"public_network": schema.StringAttribute{
				MarkdownDescription: "Assign public IP address (y or n). Defaults to n.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"dns": schema.StringAttribute{
				MarkdownDescription: "Add hostname to DNS in dev.fyre.ibm.com domain (y or n). Defaults to n.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ssh_keys": schema.ListAttribute{
				MarkdownDescription: "SSH public keys to add to the VM. Can be a list of keys.",
				Optional:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Custom password for the VM. Can be updated after creation.",
				Optional:            true,
				Sensitive:           true,
			},
			"disable_delete": schema.StringAttribute{
				MarkdownDescription: "Disable deletion of the VM to prevent accidental deletes (y or n). Can be updated after creation.",
				Optional:            true,
				Computed:            true,
			},
			"additional_disks": schema.ListAttribute{
				MarkdownDescription: "Additional disk sizes in GB (max 2). Can be added after creation but not removed.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"fqdn": schema.StringAttribute{
				MarkdownDescription: "Fully Qualified Domain Name of the VM",
				Computed:            true,
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "Current state of the VM",
				Computed:            true,
			},
			"created": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp",
				Computed:            true,
			},
			"location": schema.StringAttribute{
				MarkdownDescription: "VM location",
				Computed:            true,
			},
			"ips": schema.ListNestedAttribute{
				MarkdownDescription: "List of IP addresses assigned to the VM. These are assigned by the Fyre API and cannot be set by the user.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"ip": schema.StringAttribute{
							MarkdownDescription: "IP address",
							Computed:            true,
						},
						"type": schema.StringAttribute{
							MarkdownDescription: "IP type (private or public)",
							Computed:            true,
						},
						"scope": schema.StringAttribute{
							MarkdownDescription: "IP scope",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

// ConfigValidators returns a list of config validators for the resource.
func (r *ResourceVM) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		&quotaTypeValidator{},
	}
}

// Configure adds the provider configured client to the resource.
func (r *ResourceVM) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(*FyreProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *FyreProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = providerData.Client
	r.defaultSite = providerData.DefaultSite
	r.defaultProductGroupID = providerData.DefaultProductGroupID
}

// Create creates the resource and sets the initial Terraform state.
func (r *ResourceVM) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VMModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine site
	site := data.Site.ValueString()
	if site == "" {
		site = r.defaultSite
	}

	// Build node_array structure for VM creation
	// Note: NodeArray is an inline struct, not a named type
	nodeConfig := struct {
		AdditionalDisk *[]string                                     `json:"additional_disk,omitempty"`
		Count          *int                                          `json:"count,omitempty"`
		Cpu            *int                                          `json:"cpu,omitempty"`
		Dedicated      *client.VMCreateRequestNodeArrayDedicated     `json:"dedicated,omitempty"`
		DedicatedHost  *[]string                                     `json:"dedicated_host,omitempty"`
		Description    *string                                       `json:"description,omitempty"`
		Dns            *client.VMCreateRequestNodeArrayDns           `json:"dns,omitempty"`
		Hostname       *client.VMCreateRequest_NodeArray_Hostname    `json:"hostname,omitempty"`
		Memory         *int                                          `json:"memory,omitempty"`
		Os             string                                        `json:"os"`
		Platform       *client.VMCreateRequestNodeArrayPlatform      `json:"platform,omitempty"`
		PublicNetwork  *client.VMCreateRequestNodeArrayPublicNetwork `json:"public_network,omitempty"`
	}{}

	// Required field
	nodeConfig.Os = data.OS.ValueString()

	// Optional fields with defaults
	if !data.Platform.IsNull() {
		platform := client.VMCreateRequestNodeArrayPlatform(data.Platform.ValueString())
		nodeConfig.Platform = &platform
	}

	if !data.CPU.IsNull() {
		cpu := int(data.CPU.ValueInt64())
		nodeConfig.Cpu = &cpu
	}

	if !data.Memory.IsNull() {
		memory := int(data.Memory.ValueInt64())
		nodeConfig.Memory = &memory
	}

	if !data.Hostname.IsNull() {
		hostname := data.Hostname.ValueString()
		hostnameUnion := client.VMCreateRequest_NodeArray_Hostname{}
		if err := hostnameUnion.FromVMCreateRequestNodeArrayHostname0(hostname); err != nil {
			resp.Diagnostics.AddError("Configuration Error", fmt.Sprintf("Unable to set hostname: %s", err))
			return
		}
		nodeConfig.Hostname = &hostnameUnion
	}

	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		nodeConfig.Description = &desc
	}

	if !data.PublicNetwork.IsNull() {
		pubNet := client.VMCreateRequestNodeArrayPublicNetwork(data.PublicNetwork.ValueString())
		nodeConfig.PublicNetwork = &pubNet
	}

	if !data.DNS.IsNull() {
		dns := client.VMCreateRequestNodeArrayDns(data.DNS.ValueString())
		nodeConfig.Dns = &dns
	}

	if !data.AdditionalDisks.IsNull() {
		var disks []string
		resp.Diagnostics.Append(data.AdditionalDisks.ElementsAs(ctx, &disks, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		nodeConfig.AdditionalDisk = &disks
	}

	// Build API request
	createReq := client.VMCreateRequest{
		NodeArray: []struct {
			AdditionalDisk *[]string                                     `json:"additional_disk,omitempty"`
			Count          *int                                          `json:"count,omitempty"`
			Cpu            *int                                          `json:"cpu,omitempty"`
			Dedicated      *client.VMCreateRequestNodeArrayDedicated     `json:"dedicated,omitempty"`
			DedicatedHost  *[]string                                     `json:"dedicated_host,omitempty"`
			Description    *string                                       `json:"description,omitempty"`
			Dns            *client.VMCreateRequestNodeArrayDns           `json:"dns,omitempty"`
			Hostname       *client.VMCreateRequest_NodeArray_Hostname    `json:"hostname,omitempty"`
			Memory         *int                                          `json:"memory,omitempty"`
			Os             string                                        `json:"os"`
			Platform       *client.VMCreateRequestNodeArrayPlatform      `json:"platform,omitempty"`
			PublicNetwork  *client.VMCreateRequestNodeArrayPublicNetwork `json:"public_network,omitempty"`
		}{nodeConfig},
	}

	if !data.QuotaType.IsNull() {
		quotaType := client.VMCreateRequestQuotaType(data.QuotaType.ValueString())
		createReq.QuotaType = &quotaType
	}

	// Use product_group_id from config, or inherit from provider
	if !data.ProductGroupID.IsNull() && !data.ProductGroupID.IsUnknown() {
		pgID := data.ProductGroupID.ValueString()
		createReq.ProductGroupId = &pgID
	} else if r.defaultProductGroupID != nil {
		pgID := fmt.Sprintf("%d", *r.defaultProductGroupID)
		createReq.ProductGroupId = &pgID
	}

	if !data.TimeToLive.IsNull() {
		ttl := data.TimeToLive.ValueString()
		createReq.TimeToLive = &ttl
	}

	if !data.Expiration.IsNull() {
		exp := data.Expiration.ValueString()
		createReq.Expiration = &exp
	}

	if !data.SSHKeys.IsNull() {
		var sshKeys []string
		resp.Diagnostics.Append(data.SSHKeys.ElementsAs(ctx, &sshKeys, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		sshKeyUnion := client.VMCreateRequest_SshKey{}
		if err := sshKeyUnion.FromVMCreateRequestSshKey1(sshKeys); err != nil {
			resp.Diagnostics.AddError("Configuration Error", fmt.Sprintf("Unable to set SSH keys: %s", err))
			return
		}
		createReq.SshKey = &sshKeyUnion
	}

	tflog.Debug(ctx, "Creating VM", map[string]any{
		"os":       data.OS.ValueString(),
		"platform": data.Platform.ValueString(),
	})

	// Call API with retry logic
	siteParam := client.CreateVMParamsSite(site)
	var createResp *client.CreateVMResponse
	err := retryWithBackoff(ctx, "Create VM", 2, nil, func() (*http.Response, *client.Error, error) {
		var callErr error
		createResp, callErr = r.client.CreateVMWithResponse(ctx, &client.CreateVMParams{
			Site: &siteParam,
		}, createReq)
		if callErr != nil {
			return nil, nil, callErr
		}
		if createResp.StatusCode() < 200 || createResp.StatusCode() >= 300 {
			return createResp.HTTPResponse, createResp.JSON400, fmt.Errorf("API returned status %d: %s", createResp.StatusCode(), string(createResp.Body))
		}
		return createResp.HTTPResponse, nil, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	if createResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Parse Error",
			fmt.Sprintf("Unable to parse create response. Body: %s", string(createResp.Body)),
		)
		return
	}

	// The create response contains vm_id and request_id
	vmID := ""
	if createResp.JSON200.VmId != nil {
		vmID = *createResp.JSON200.VmId
	}

	requestID := ""
	if createResp.JSON200.RequestId != nil {
		requestID = *createResp.JSON200.RequestId
	}

	if vmID == "" || requestID == "" {
		resp.Diagnostics.AddError(
			"API Error",
			"VM creation response did not include vm_id or request_id",
		)
		return
	}

	tflog.Debug(ctx, "VM creation initiated, polling for completion", map[string]any{
		"vm_id":      vmID,
		"request_id": requestID,
	})

	// Poll for request completion (max 10 minutes, check every 30 seconds after 2 minute initial delay)
	pollCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	poller := newRequestPoller(
		r.client,
		requestID,
		site,
		"VM Build",
		newFixedIntervalStrategy(30*time.Second),
		withInitialDelay(2*time.Minute),
	)

	if err := poller.poll(pollCtx); err != nil {
		resp.Diagnostics.AddError("VM Build Failed", err.Error())
		return
	}

	// Now read the VM details to populate full state
	readSite := client.GetVMDetailsParamsSite(site)
	var vmResp *client.GetVMDetailsResponse
	err = retryWithBackoff(ctx, "Get VM Details", 1, nil, func() (*http.Response, *client.Error, error) {
		var callErr error
		vmResp, callErr = r.client.GetVMDetailsWithResponse(ctx, vmID, &client.GetVMDetailsParams{
			Site: &readSite,
		})
		if callErr != nil {
			return nil, nil, callErr
		}
		if vmResp.StatusCode() < 200 || vmResp.StatusCode() >= 300 {
			return vmResp.HTTPResponse, nil, fmt.Errorf("API returned status %d", vmResp.StatusCode())
		}
		if vmResp.JSON200 == nil {
			return vmResp.HTTPResponse, nil, fmt.Errorf("no response payload returned")
		}
		return vmResp.HTTPResponse, nil, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read created VM: %s", err))
		return
	}

	vmDetails := vmResp.JSON200

	// Map response to state
	data.ID = types.StringValue(vmID)
	data.VMID = types.StringValue(vmID)
	data.Site = types.StringValue(site)

	if vmDetails.Location != nil {
		data.Location = types.StringValue(*vmDetails.Location)
	}

	// Set hostname from API response (either user-provided or auto-generated)
	if vmDetails.Hostname != nil {
		data.Hostname = types.StringValue(*vmDetails.Hostname)
	} else if data.Hostname.IsUnknown() {
		data.Hostname = types.StringNull()
	}

	// Only set optional user-settable fields if user provided them or they're required
	// This prevents plan inconsistency when API returns defaults for unset optional fields
	if !data.Description.IsNull() && vmDetails.Description != nil {
		data.Description = types.StringValue(*vmDetails.Description)
	} else if data.Description.IsUnknown() {
		data.Description = types.StringNull()
	}

	// Platform is optional with default, always set from API
	if vmDetails.Platform != nil {
		data.Platform = types.StringValue(*vmDetails.Platform)
	} else if data.Platform.IsUnknown() {
		data.Platform = types.StringNull()
	}

	// OS is required, always set from API
	if vmDetails.Os != nil {
		data.OS = types.StringValue(*vmDetails.Os)
	}

	// CPU and Memory are optional with defaults, always set from API
	if vmDetails.Cpu != nil {
		data.CPU = types.Int64Value(int64(*vmDetails.Cpu))
	} else if data.CPU.IsUnknown() {
		data.CPU = types.Int64Null()
	}

	if vmDetails.Memory != nil {
		data.Memory = types.Int64Value(int64(*vmDetails.Memory))
	} else if data.Memory.IsUnknown() {
		data.Memory = types.Int64Null()
	}

	// Computed fields - always set from API
	if vmDetails.State != nil {
		data.State = types.StringValue(*vmDetails.State)
	} else if data.State.IsUnknown() {
		data.State = types.StringNull()
	}

	if vmDetails.Fqdn != nil {
		data.FQDN = types.StringValue(*vmDetails.Fqdn)
	} else if data.FQDN.IsUnknown() {
		data.FQDN = types.StringNull()
	}

	if vmDetails.Created != nil {
		data.Created = types.StringValue(*vmDetails.Created)
	} else if data.Created.IsUnknown() {
		data.Created = types.StringNull()
	}

	// DisableDelete is optional, only set if user provided it
	if !data.DisableDelete.IsNull() && vmDetails.DisableDelete != nil {
		data.DisableDelete = types.StringValue(*vmDetails.DisableDelete)
	} else if data.DisableDelete.IsUnknown() {
		data.DisableDelete = types.StringNull()
	}

	// Set API-returned expiration timestamp to expiration_time
	if vmDetails.Expiration != nil {
		data.ExpirationTime = types.StringValue(*vmDetails.Expiration)
	} else {
		data.ExpirationTime = types.StringNull()
	}

	// Set quota_type from API if not provided by user (optional+computed with default)
	if data.QuotaType.IsUnknown() {
		if vmDetails.QuotaType != nil {
			data.QuotaType = types.StringValue(*vmDetails.QuotaType)
		} else {
			data.QuotaType = types.StringNull()
		}
	}

	// Set product_group_id from API if not provided by user (optional+computed with default)
	if data.ProductGroupID.IsUnknown() {
		if vmDetails.ProductGroupId != nil {
			data.ProductGroupID = types.StringValue(fmt.Sprintf("%d", *vmDetails.ProductGroupId))
		} else {
			data.ProductGroupID = types.StringNull()
		}
	}

	// Set remaining computed optional fields to null if unknown
	if data.DNS.IsUnknown() {
		data.DNS = types.StringNull()
	}
	if data.PublicNetwork.IsUnknown() {
		data.PublicNetwork = types.StringNull()
	}
	if data.Expiration.IsUnknown() {
		data.Expiration = types.StringNull()
	}
	if data.ExpirationTime.IsUnknown() {
		data.ExpirationTime = types.StringNull()
	}
	if data.AdditionalDisks.IsUnknown() {
		data.AdditionalDisks = types.ListNull(types.StringType)
	}
	if data.Location.IsUnknown() {
		data.Location = types.StringNull()
	}

	// Map IPs from API response
	if vmDetails.Ips != nil && len(*vmDetails.Ips) > 0 {
		ipElements := make([]attr.Value, 0, len(*vmDetails.Ips))
		for _, ip := range *vmDetails.Ips {
			ipAttrs := map[string]attr.Value{
				"ip":    types.StringNull(),
				"type":  types.StringNull(),
				"scope": types.StringNull(),
			}
			if ip.Ip != nil {
				ipAttrs["ip"] = types.StringValue(*ip.Ip)
			}
			if ip.Type != nil {
				ipAttrs["type"] = types.StringValue(*ip.Type)
			}
			if ip.Scope != nil {
				ipAttrs["scope"] = types.StringValue(*ip.Scope)
			}
			ipObj, diags := types.ObjectValue(
				map[string]attr.Type{
					"ip":    types.StringType,
					"type":  types.StringType,
					"scope": types.StringType,
				},
				ipAttrs,
			)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			ipElements = append(ipElements, ipObj)
		}
		ipsList, diags := types.ListValue(
			types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"ip":    types.StringType,
					"type":  types.StringType,
					"scope": types.StringType,
				},
			},
			ipElements,
		)
		resp.Diagnostics.Append(diags...)
		if !resp.Diagnostics.HasError() {
			data.IPs = ipsList
		}
	} else {
		data.IPs = types.ListNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"ip":    types.StringType,
				"type":  types.StringType,
				"scope": types.StringType,
			},
		})
	}

	tflog.Trace(ctx, "created VM resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *ResourceVM) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VMModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use vm_id if available, otherwise use id
	vmIdentifier := data.VMID.ValueString()
	if vmIdentifier == "" {
		vmIdentifier = data.ID.ValueString()
	}

	// Determine site
	site := client.GetVMDetailsParamsSite(data.Site.ValueString())
	if site == "" {
		site = client.GetVMDetailsParamsSite(r.defaultSite)
	}

	// Call API with retry logic
	var readResp *client.GetVMDetailsResponse
	err := retryWithBackoff(ctx, "Get VM Details", 1, nil, func() (*http.Response, *client.Error, error) {
		var callErr error
		readResp, callErr = r.client.GetVMDetailsWithResponse(ctx, vmIdentifier, &client.GetVMDetailsParams{
			Site: &site,
		})
		if callErr != nil {
			return nil, nil, callErr
		}
		// Don't retry on 404 - VM was deleted
		if readResp.StatusCode() == 404 {
			return readResp.HTTPResponse, nil, nil
		}
		if readResp.StatusCode() < 200 || readResp.StatusCode() >= 300 {
			return readResp.HTTPResponse, nil, fmt.Errorf("API returned status %d: %s", readResp.StatusCode(), string(readResp.Body))
		}
		return readResp.HTTPResponse, nil, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	if readResp.StatusCode() == 404 {
		// VM no longer exists, remove from state
		resp.State.RemoveResource(ctx)
		return
	}

	if readResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Parse Error",
			fmt.Sprintf("Unable to parse read response. Body: %s", string(readResp.Body)),
		)
		return
	}

	vmDetails := readResp.JSON200

	// Check if VM is in deleted state - treat same as 404
	if vmDetails.State != nil &&
		(*vmDetails.State == "deleting" || *vmDetails.State == "deleted") {
		tflog.Debug(ctx, "VM is in deleted state, removing from Terraform state", map[string]any{
			"vm_id": vmIdentifier,
		})
		resp.State.RemoveResource(ctx)
		return
	}

	// Update state with latest data
	if vmDetails.VmId != nil {
		data.VMID = types.StringValue(*vmDetails.VmId)
		data.ID = types.StringValue(*vmDetails.VmId)
	}

	if vmDetails.Location != nil {
		data.Location = types.StringValue(*vmDetails.Location)
	}

	if vmDetails.Hostname != nil {
		data.Hostname = types.StringValue(*vmDetails.Hostname)
	}

	if vmDetails.Description != nil {
		data.Description = types.StringValue(*vmDetails.Description)
	} else {
		data.Description = types.StringNull()
	}

	if vmDetails.Platform != nil {
		data.Platform = types.StringValue(*vmDetails.Platform)
	}

	if vmDetails.Os != nil {
		data.OS = types.StringValue(*vmDetails.Os)
	}

	if vmDetails.Cpu != nil {
		data.CPU = types.Int64Value(int64(*vmDetails.Cpu))
	}

	if vmDetails.Memory != nil {
		data.Memory = types.Int64Value(int64(*vmDetails.Memory))
	}

	if vmDetails.State != nil {
		data.State = types.StringValue(*vmDetails.State)
	}

	if vmDetails.Fqdn != nil {
		data.FQDN = types.StringValue(*vmDetails.Fqdn)
	}

	if vmDetails.Created != nil {
		data.Created = types.StringValue(*vmDetails.Created)
	}

	if vmDetails.DisableDelete != nil {
		data.DisableDelete = types.StringValue(*vmDetails.DisableDelete)
	}

	// Set quota_type from API response
	if vmDetails.QuotaType != nil {
		data.QuotaType = types.StringValue(*vmDetails.QuotaType)
	}

	// Set product_group_id from API response
	if vmDetails.ProductGroupId != nil {
		data.ProductGroupID = types.StringValue(fmt.Sprintf("%d", *vmDetails.ProductGroupId))
	}

	// Set site from current state or default
	if data.Site.IsNull() || data.Site.IsUnknown() {
		data.Site = types.StringValue(string(site))
	}

	// Map additional_disks from API response
	if vmDetails.AdditionalDisk != nil && len(*vmDetails.AdditionalDisk) > 0 {
		disksList := make([]attr.Value, 0, len(*vmDetails.AdditionalDisk))
		for _, diskSize := range *vmDetails.AdditionalDisk {
			disksList = append(disksList, types.StringValue(fmt.Sprintf("%d", diskSize)))
		}
		if len(disksList) > 0 {
			listValue, diags := types.ListValue(types.StringType, disksList)
			resp.Diagnostics.Append(diags...)
			if !resp.Diagnostics.HasError() {
				data.AdditionalDisks = listValue
			}
		}
	} else {
		data.AdditionalDisks = types.ListNull(types.StringType)
	}

	// Map API expiration timestamp to expiration_time (computed field)
	if vmDetails.Expiration != nil {
		data.ExpirationTime = types.StringValue(*vmDetails.Expiration)
	} else {
		data.ExpirationTime = types.StringNull()
	}

	// The expiration field is user input (relative time), not returned by API
	// During import/read, we don't have the original user input, so set to null
	// This allows users to set it after import if needed
	if data.Expiration.IsUnknown() {
		data.Expiration = types.StringNull()
	}

	// Map IPs from API response
	if vmDetails.Ips != nil && len(*vmDetails.Ips) > 0 {
		ipElements := make([]attr.Value, 0, len(*vmDetails.Ips))
		for _, ip := range *vmDetails.Ips {
			ipAttrs := map[string]attr.Value{
				"ip":    types.StringNull(),
				"type":  types.StringNull(),
				"scope": types.StringNull(),
			}
			if ip.Ip != nil {
				ipAttrs["ip"] = types.StringValue(*ip.Ip)
			}
			if ip.Type != nil {
				ipAttrs["type"] = types.StringValue(*ip.Type)
			}
			if ip.Scope != nil {
				ipAttrs["scope"] = types.StringValue(*ip.Scope)
			}
			ipObj, diags := types.ObjectValue(
				map[string]attr.Type{
					"ip":    types.StringType,
					"type":  types.StringType,
					"scope": types.StringType,
				},
				ipAttrs,
			)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			ipElements = append(ipElements, ipObj)
		}
		ipsList, diags := types.ListValue(
			types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"ip":    types.StringType,
					"type":  types.StringType,
					"scope": types.StringType,
				},
			},
			ipElements,
		)
		resp.Diagnostics.Append(diags...)
		if !resp.Diagnostics.HasError() {
			data.IPs = ipsList
		}
	} else {
		data.IPs = types.ListNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"ip":    types.StringType,
				"type":  types.StringType,
				"scope": types.StringType,
			},
		})
	}

	tflog.Trace(ctx, "read VM resource")

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *ResourceVM) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state VMModel

	// Read Terraform plan and state data
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vmIdentifier := state.VMID.ValueString()
	if vmIdentifier == "" {
		vmIdentifier = state.ID.ValueString()
	}

	siteStr := state.Site.ValueString()
	if siteStr == "" {
		siteStr = r.defaultSite
	}

	// Handle CPU/Memory updates
	if !plan.CPU.Equal(state.CPU) || !plan.Memory.Equal(state.Memory) {
		site := client.ModifyVMResourcesParamsSite(siteStr)
		updateReq := client.VMResourceUpdate{}

		if !plan.CPU.IsNull() {
			cpu := int(plan.CPU.ValueInt64())
			updateReq.Cpu = &cpu
		}

		if !plan.Memory.IsNull() {
			memory := int(plan.Memory.ValueInt64())
			updateReq.Memory = &memory
		}

		tflog.Debug(ctx, "Updating VM resources", map[string]any{
			"vm_id":  vmIdentifier,
			"cpu":    plan.CPU.ValueInt64(),
			"memory": plan.Memory.ValueInt64(),
		})

		var updateResp *client.ModifyVMResourcesResponse
		err := retryWithBackoff(ctx, "Modify VM Resources", 2, nil, func() (*http.Response, *client.Error, error) {
			var callErr error
			updateResp, callErr = r.client.ModifyVMResourcesWithResponse(ctx, vmIdentifier, &client.ModifyVMResourcesParams{
				Site: &site,
			}, updateReq)
			if callErr != nil {
				return nil, nil, callErr
			}
			if updateResp.StatusCode() < 200 || updateResp.StatusCode() >= 300 {
				return updateResp.HTTPResponse, updateResp.JSON400, fmt.Errorf("API returned status %d: %s", updateResp.StatusCode(), string(updateResp.Body))
			}
			return updateResp.HTTPResponse, nil, nil
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", err.Error())
			return
		}

		// Poll for resource update completion if request_id is returned
		if updateResp.JSON200 != nil && updateResp.JSON200.RequestId != nil {
			requestID := *updateResp.JSON200.RequestId
			tflog.Debug(ctx, "Polling resource update request", map[string]any{"request_id": requestID})

			pollCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()

			poller := newRequestPoller(
				r.client,
				requestID,
				siteStr,
				"Modify VM Resources",
				newFixedIntervalStrategy(30*time.Second),
				withInitialDelay(15*time.Second),
			)

			if err := poller.poll(pollCtx); err != nil {
				resp.Diagnostics.AddError("Modify VM Resources Failed", err.Error())
				return
			}
		}
	}

	// Handle hostname update
	if !plan.Hostname.Equal(state.Hostname) && !plan.Hostname.IsNull() {
		hostnameSite := client.UpdateVMHostnameParamsSite(siteStr)
		hostnameReq := client.VMHostnameUpdate{
			Name: plan.Hostname.ValueString(),
		}

		tflog.Debug(ctx, "Updating VM hostname", map[string]any{
			"vm_id":        vmIdentifier,
			"new_hostname": plan.Hostname.ValueString(),
		})

		var hostnameResp *client.UpdateVMHostnameResponse
		err := retryWithBackoff(ctx, "Update VM Hostname", 2, nil, func() (*http.Response, *client.Error, error) {
			var callErr error
			hostnameResp, callErr = r.client.UpdateVMHostnameWithResponse(ctx, vmIdentifier, &client.UpdateVMHostnameParams{
				Site: &hostnameSite,
			}, hostnameReq)
			if callErr != nil {
				return nil, nil, callErr
			}
			if hostnameResp.StatusCode() < 200 || hostnameResp.StatusCode() >= 300 {
				return hostnameResp.HTTPResponse, nil, fmt.Errorf("API returned status %d: %s", hostnameResp.StatusCode(), string(hostnameResp.Body))
			}
			if hostnameResp.JSON200 == nil {
				return hostnameResp.HTTPResponse, nil, fmt.Errorf("no response payload returned")
			}
			return hostnameResp.HTTPResponse, nil, nil
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", err.Error())
			return
		}

		// Poll request until completion
		if hostnameResp.JSON200.RequestId != nil {
			requestID := *hostnameResp.JSON200.RequestId
			tflog.Debug(ctx, "Polling change hostname request", map[string]any{"request_id": requestID})

			pollCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()

			poller := newRequestPoller(
				r.client,
				requestID,
				siteStr,
				"Change Hostname",
				newFixedIntervalStrategy(30*time.Second),
				withInitialDelay(15*time.Second),
			)

			if err := poller.poll(pollCtx); err != nil {
				resp.Diagnostics.AddError("Change Hostname Failed", err.Error())
				return
			}
		}
	}

	// Handle description update
	if !plan.Description.Equal(state.Description) {
		descSite := client.UpdateVMDescriptionParamsSite(siteStr)
		descReq := client.VMDescriptionUpdate{
			Description: plan.Description.ValueString(),
		}

		var descResp *client.UpdateVMDescriptionResponse
		err := retryWithBackoff(ctx, "Update VM Description", 2, nil, func() (*http.Response, *client.Error, error) {
			var callErr error
			descResp, callErr = r.client.UpdateVMDescriptionWithResponse(ctx, vmIdentifier, &client.UpdateVMDescriptionParams{
				Site: &descSite,
			}, descReq)
			if callErr != nil {
				return nil, nil, callErr
			}
			if descResp.StatusCode() < 200 || descResp.StatusCode() >= 300 {
				return descResp.HTTPResponse, nil, fmt.Errorf("API returned status %d: %s", descResp.StatusCode(), string(descResp.Body))
			}
			return descResp.HTTPResponse, nil, nil
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", err.Error())
			return
		}

		if descResp.StatusCode() != 200 {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("API returned status %d: %s", descResp.StatusCode(), string(descResp.Body)))
			return
		}

		// Poll for description update completion if request_id is returned
		if descResp.JSON200 != nil && descResp.JSON200.RequestId != nil {
			rid := *descResp.JSON200.RequestId
			tflog.Debug(ctx, "Polling description update request", map[string]any{"request_id": rid})

			pollCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()

			poller := newRequestPoller(
				r.client,
				rid,
				siteStr,
				"Update VM Description",
				newFixedIntervalStrategy(30*time.Second),
				withInitialDelay(15*time.Second),
			)

			if err := poller.poll(pollCtx); err != nil {
				resp.Diagnostics.AddError("Update VM Description Failed", err.Error())
				return
			}
		}
	}

	// Handle expiration update
	if !plan.Expiration.Equal(state.Expiration) && !plan.Expiration.IsNull() {
		expSite := client.UpdateVMExpirationParamsSite(siteStr)
		expReq := client.ExpirationUpdate{
			Expiration: plan.Expiration.ValueString(),
		}

		var expResp *client.UpdateVMExpirationResponse
		err := retryWithBackoff(ctx, "Update VM Expiration", 2, nil, func() (*http.Response, *client.Error, error) {
			var callErr error
			expResp, callErr = r.client.UpdateVMExpirationWithResponse(ctx, vmIdentifier, &client.UpdateVMExpirationParams{
				Site: &expSite,
			}, expReq)
			if callErr != nil {
				return nil, nil, callErr
			}
			if expResp.StatusCode() < 200 || expResp.StatusCode() >= 300 {
				return expResp.HTTPResponse, expResp.JSON400, fmt.Errorf("API returned status %d: %s", expResp.StatusCode(), string(expResp.Body))
			}
			return expResp.HTTPResponse, nil, nil
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update VM expiration: %s", err))
			return
		}

		if expResp.StatusCode() != 200 {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("API returned status %d: %s", expResp.StatusCode(), string(expResp.Body)))
			return
		}

		// Poll for expiration update completion if request_id is returned
		if expResp.JSON200 != nil && expResp.JSON200.RequestId != nil {
			requestID := *expResp.JSON200.RequestId
			tflog.Debug(ctx, "Polling expiration update request", map[string]any{"request_id": requestID})

			pollCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()

			poller := newRequestPoller(
				r.client,
				requestID,
				siteStr,
				"Change VM Expiration",
				newFixedIntervalStrategy(30*time.Second),
				withInitialDelay(15*time.Second),
			)

			if err := poller.poll(pollCtx); err != nil {
				resp.Diagnostics.AddError("Change VM Expiration Failed", err.Error())
				return
			}
		}
	}

	// Handle password update
	if !plan.Password.Equal(state.Password) && !plan.Password.IsNull() {
		pwdSite := client.ChangeVMPasswordParamsSite(siteStr)
		pwdReq := client.VMPasswordUpdate{
			Password: plan.Password.ValueString(),
		}

		var pwdResp *client.ChangeVMPasswordResponse
		err := retryWithBackoff(ctx, "Change VM Password", 2, nil, func() (*http.Response, *client.Error, error) {
			var callErr error
			pwdResp, callErr = r.client.ChangeVMPasswordWithResponse(ctx, vmIdentifier, &client.ChangeVMPasswordParams{
				Site: &pwdSite,
			}, pwdReq)
			if callErr != nil {
				return nil, nil, callErr
			}
			if pwdResp.StatusCode() < 200 || pwdResp.StatusCode() >= 300 {
				return pwdResp.HTTPResponse, nil, fmt.Errorf("API returned status %d: %s", pwdResp.StatusCode(), string(pwdResp.Body))
			}
			return pwdResp.HTTPResponse, nil, nil
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", err.Error())
			return
		}
	}

	// Handle disable_delete toggle
	if !plan.DisableDelete.Equal(state.DisableDelete) && !plan.DisableDelete.IsNull() {
		if plan.DisableDelete.ValueString() == "y" {
			disableSite := client.DisableVMDeleteParamsSite(siteStr)

			var disableResp *client.DisableVMDeleteResponse
			err := retryWithBackoff(ctx, "Disable VM Delete", 2, nil, func() (*http.Response, *client.Error, error) {
				var callErr error
				disableResp, callErr = r.client.DisableVMDeleteWithResponse(ctx, vmIdentifier, &client.DisableVMDeleteParams{
					Site: &disableSite,
				})
				if callErr != nil {
					return nil, nil, callErr
				}
				if disableResp.StatusCode() < 200 || disableResp.StatusCode() >= 300 {
					return disableResp.HTTPResponse, nil, fmt.Errorf("API returned status %d: %s", disableResp.StatusCode(), string(disableResp.Body))
				}
				return disableResp.HTTPResponse, nil, nil
			})
			if err != nil {
				resp.Diagnostics.AddError("Client Error", err.Error())
				return
			}

			// Poll for disable delete completion if request_id is returned
			if disableResp.JSON200 != nil && disableResp.JSON200.RequestId != nil {
				requestID := *disableResp.JSON200.RequestId
				tflog.Debug(ctx, "Polling disable delete request", map[string]any{"request_id": requestID})

				pollCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
				defer cancel()

				poller := newRequestPoller(
					r.client,
					requestID,
					siteStr,
					"Disable Delete Update",
					newFixedIntervalStrategy(30*time.Second),
					withInitialDelay(15*time.Second),
				)

				if err := poller.poll(pollCtx); err != nil {
					resp.Diagnostics.AddError("Disable Delete Failed", err.Error())
					return
				}
			}
		} else {
			enableSite := client.EnableVMDeleteParamsSite(siteStr)

			var enableResp *client.EnableVMDeleteResponse
			err := retryWithBackoff(ctx, "Enable VM Delete", 2, nil, func() (*http.Response, *client.Error, error) {
				var callErr error
				enableResp, callErr = r.client.EnableVMDeleteWithResponse(ctx, vmIdentifier, &client.EnableVMDeleteParams{
					Site: &enableSite,
				})
				if callErr != nil {
					return nil, nil, callErr
				}
				if enableResp.StatusCode() < 200 || enableResp.StatusCode() >= 300 {
					return enableResp.HTTPResponse, nil, fmt.Errorf("API returned status %d: %s", enableResp.StatusCode(), string(enableResp.Body))
				}
				return enableResp.HTTPResponse, nil, nil
			})
			if err != nil {
				resp.Diagnostics.AddError("Client Error", err.Error())
				return
			}

			// Poll for enable delete completion if request_id is returned
			if enableResp.JSON200 != nil && enableResp.JSON200.RequestId != nil {
				requestID := *enableResp.JSON200.RequestId
				tflog.Debug(ctx, "Polling enable delete request", map[string]any{"request_id": requestID})

				pollCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
				defer cancel()

				poller := newRequestPoller(
					r.client,
					requestID,
					siteStr,
					"Enable Delete Update",
					newFixedIntervalStrategy(30*time.Second),
					withInitialDelay(15*time.Second),
				)

				if err := poller.poll(pollCtx); err != nil {
					resp.Diagnostics.AddError("Enable Delete Failed", err.Error())
					return
				}
			}
		}
	}

	// Handle additional disks (can only add, not remove)
	if !plan.AdditionalDisks.Equal(state.AdditionalDisks) && !plan.AdditionalDisks.IsNull() {
		var planDisks, stateDisks []string
		resp.Diagnostics.Append(plan.AdditionalDisks.ElementsAs(ctx, &planDisks, false)...)
		if !state.AdditionalDisks.IsNull() {
			resp.Diagnostics.Append(state.AdditionalDisks.ElementsAs(ctx, &stateDisks, false)...)
		}

		if resp.Diagnostics.HasError() {
			return
		}

		// Only add new disks (can't remove existing ones)
		if len(planDisks) > len(stateDisks) {
			newDisks := planDisks[len(stateDisks):]
			// Convert string disk sizes to integers
			newDisksInt := make([]int, len(newDisks))
			for i, disk := range newDisks {
				var size int
				if _, err := fmt.Sscanf(disk, "%d", &size); err != nil {
					resp.Diagnostics.AddError("Planning Disk", err.Error())
					return
				}
				newDisksInt[i] = size
			}

			diskSite := client.AddVMDiskParamsSite(siteStr)
			diskReq := client.VMDiskAdd{
				AdditionalDisk: newDisksInt,
			}

			var diskResp *client.AddVMDiskResponse
			err := retryWithBackoff(ctx, "Add VM Disk", 2, nil, func() (*http.Response, *client.Error, error) {
				var callErr error
				diskResp, callErr = r.client.AddVMDiskWithResponse(ctx, vmIdentifier, &client.AddVMDiskParams{
					Site: &diskSite,
				}, diskReq)
				if callErr != nil {
					return nil, nil, callErr
				}
				if diskResp.StatusCode() < 200 || diskResp.StatusCode() >= 300 {
					return diskResp.HTTPResponse, diskResp.JSON400, fmt.Errorf("API returned status %d: %s", diskResp.StatusCode(), string(diskResp.Body))
				}
				return diskResp.HTTPResponse, nil, nil
			})
			if err != nil {
				resp.Diagnostics.AddError("Client Error", err.Error())
				return
			}
		}
	}

	tflog.Trace(ctx, "updated VM resource")

	// Read the updated VM state
	readSite := client.GetVMDetailsParamsSite(siteStr)
	var readResp *client.GetVMDetailsResponse
	err := retryWithBackoff(ctx, "Get VM Details", 1, nil, func() (*http.Response, *client.Error, error) {
		var callErr error
		readResp, callErr = r.client.GetVMDetailsWithResponse(ctx, vmIdentifier, &client.GetVMDetailsParams{
			Site: &readSite,
		})
		if callErr != nil {
			return nil, nil, callErr
		}
		if readResp.StatusCode() < 200 || readResp.StatusCode() >= 300 {
			return readResp.HTTPResponse, nil, fmt.Errorf("API returned status %d", readResp.StatusCode())
		}
		if readResp.JSON200 == nil {
			return readResp.HTTPResponse, nil, fmt.Errorf("no response payload returned")
		}
		return readResp.HTTPResponse, nil, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read updated VM: %s", err))
		return
	}

	// Update plan with latest state from API
	vmDetails := readResp.JSON200

	// Update mutable fields from API
	if vmDetails.Cpu != nil {
		plan.CPU = types.Int64Value(int64(*vmDetails.Cpu))
	}

	if vmDetails.Memory != nil {
		plan.Memory = types.Int64Value(int64(*vmDetails.Memory))
	}

	if vmDetails.Hostname != nil {
		plan.Hostname = types.StringValue(*vmDetails.Hostname)
	}

	if vmDetails.Description != nil {
		plan.Description = types.StringValue(*vmDetails.Description)
	}

	if vmDetails.DisableDelete != nil {
		plan.DisableDelete = types.StringValue(*vmDetails.DisableDelete)
	}

	// Update all computed fields - these must always be set to avoid "unknown after apply" errors
	// If API doesn't return a value, preserve the existing state value (don't set to null)
	if vmDetails.State != nil {
		plan.State = types.StringValue(*vmDetails.State)
	} else if plan.State.IsUnknown() {
		// Only set to null if it was unknown, otherwise preserve existing value
		plan.State = state.State
	}

	if vmDetails.Fqdn != nil {
		plan.FQDN = types.StringValue(*vmDetails.Fqdn)
	} else if plan.FQDN.IsUnknown() {
		plan.FQDN = state.FQDN
	}

	if vmDetails.Created != nil {
		plan.Created = types.StringValue(*vmDetails.Created)
	} else if plan.Created.IsUnknown() {
		plan.Created = state.Created
	}

	if vmDetails.Location != nil {
		plan.Location = types.StringValue(*vmDetails.Location)
	} else if plan.Location.IsUnknown() {
		plan.Location = state.Location
	}

	// Map API expiration timestamp to expiration_time (computed field)
	if vmDetails.Expiration != nil {
		plan.ExpirationTime = types.StringValue(*vmDetails.Expiration)
	} else if plan.ExpirationTime.IsUnknown() {
		plan.ExpirationTime = state.ExpirationTime
	}

	// Map IPs from API response
	if vmDetails.Ips != nil && len(*vmDetails.Ips) > 0 {
		ipElements := make([]attr.Value, 0, len(*vmDetails.Ips))
		for _, ip := range *vmDetails.Ips {
			ipAttrs := map[string]attr.Value{
				"ip":    types.StringNull(),
				"type":  types.StringNull(),
				"scope": types.StringNull(),
			}
			if ip.Ip != nil {
				ipAttrs["ip"] = types.StringValue(*ip.Ip)
			}
			if ip.Type != nil {
				ipAttrs["type"] = types.StringValue(*ip.Type)
			}
			if ip.Scope != nil {
				ipAttrs["scope"] = types.StringValue(*ip.Scope)
			}
			ipObj, diags := types.ObjectValue(
				map[string]attr.Type{
					"ip":    types.StringType,
					"type":  types.StringType,
					"scope": types.StringType,
				},
				ipAttrs,
			)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			ipElements = append(ipElements, ipObj)
		}
		ipsList, diags := types.ListValue(
			types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"ip":    types.StringType,
					"type":  types.StringType,
					"scope": types.StringType,
				},
			},
			ipElements,
		)
		resp.Diagnostics.Append(diags...)
		if !resp.Diagnostics.HasError() {
			plan.IPs = ipsList
		}
	} else if plan.IPs.IsUnknown() {
		plan.IPs = types.ListNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"ip":    types.StringType,
				"type":  types.StringType,
				"scope": types.StringType,
			},
		})
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *ResourceVM) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VMModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vmIdentifier := data.VMID.ValueString()
	if vmIdentifier == "" {
		vmIdentifier = data.ID.ValueString()
	}

	siteStr := data.Site.ValueString()
	if siteStr == "" {
		siteStr = r.defaultSite
	}

	site := client.DeleteVMParamsSite(siteStr)

	tflog.Debug(ctx, "Deleting VM", map[string]any{
		"vm_id": vmIdentifier,
	})

	// First, check if the VM is already deleted by reading its current state
	readSite := client.GetVMDetailsParamsSite(siteStr)
	var readResp *client.GetVMDetailsResponse
	readErr := retryWithBackoff(ctx, "Get VM Details", 1, nil, func() (*http.Response, *client.Error, error) {
		var callErr error
		readResp, callErr = r.client.GetVMDetailsWithResponse(ctx, vmIdentifier, &client.GetVMDetailsParams{
			Site: &readSite,
		})
		if callErr != nil {
			return nil, nil, callErr
		}
		// Don't retry on 404 - VM was already deleted
		if readResp.StatusCode() == 404 {
			return readResp.HTTPResponse, nil, nil
		}
		if readResp.StatusCode() < 200 || readResp.StatusCode() >= 300 {
			return readResp.HTTPResponse, nil, fmt.Errorf("API returned status %d: %s", readResp.StatusCode(), string(readResp.Body))
		}
		return readResp.HTTPResponse, nil, nil
	})

	// If we got a 404 or the VM is in deleting/deleted state, it's already gone
	if readErr == nil && readResp != nil {
		if readResp.StatusCode() == 404 {
			tflog.Debug(ctx, "VM already deleted (404), skipping deletion", map[string]any{
				"vm_id": vmIdentifier,
			})
			tflog.Trace(ctx, "deleted VM resource")
			return
		}

		if readResp.JSON200 != nil && readResp.JSON200.State != nil &&
			(*readResp.JSON200.State == "deleted" || *readResp.JSON200.State == "deleting") {
			tflog.Debug(ctx, "VM already in deleted state, skipping deletion", map[string]any{
				"vm_id": vmIdentifier,
				"state": *readResp.JSON200.State,
			})
			tflog.Trace(ctx, "deleted VM resource")
			return
		}
	}

	// VM deletion can return 400 when VM is in some undeleteable state. There is
	// nothing in the VM state upon which to poll to determine this. I verified
	// the status in a loop that polls status and attempts to delete. The only
	// option we have is simply attempting to delete the VM and getting a 400.
	// In practice this will probably be a rare condition but in tests where we
	// create/update/delete VM's in quick order we hit it every time.
	deleteCtx, deleteCancel := context.WithTimeout(ctx, 2*time.Minute)
	defer deleteCancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var deleteResp *client.DeleteVMResponse
	var lastErr error

	for {
		deleteResp, lastErr = r.client.DeleteVMWithResponse(deleteCtx, vmIdentifier, &client.DeleteVMParams{
			Site: &site,
		})

		if lastErr != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Delete VM: %s", lastErr.Error()))
			return
		}

		// Success
		if deleteResp.StatusCode() >= 200 && deleteResp.StatusCode() < 300 {
			break
		}

		// Retry on 400 (VM in transitional state)
		if deleteResp.StatusCode() == 400 {
			tflog.Debug(ctx, "VM deletion returned 400, will retry", map[string]any{
				"vm_id": vmIdentifier,
				"body":  string(deleteResp.Body),
			})

			select {
			case <-deleteCtx.Done():
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Delete VM timed out after 2 minutes: %s", string(deleteResp.Body)))
				return
			case <-ticker.C:
				continue
			}
		}

		// Non-400 error - don't retry
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Delete VM: API returned status %d: %s", deleteResp.StatusCode(), string(deleteResp.Body)))
		return
	}

	if deleteResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"API Error",
			fmt.Sprintf("API returned status %d: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}

	if deleteResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Parse Error",
			fmt.Sprintf("Unable to parse delete response. Body: %s", string(deleteResp.Body)),
		)
		return
	}
}

// ImportState imports an existing resource into Terraform state.
func (r *ResourceVM) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import using vm_id, IP, or FQDN
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)

	// After import, the Read method will populate all other fields
	tflog.Debug(ctx, "Imported VM", map[string]any{
		"id": req.ID,
	})
}

// Description returns a plain text description of the validator's behavior.
func (v quotaTypeValidator) Description(ctx context.Context) string {
	return "Validates that time_to_live is set when quota_type is 'quick_burn' and product_group_id is set when quota_type is 'product_group'"
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior.
func (v quotaTypeValidator) MarkdownDescription(ctx context.Context) string {
	return "Validates that `time_to_live` is set when `quota_type` is `quick_burn` and `product_group_id` is set when `quota_type` is `product_group`"
}

// ValidateResource performs the validation.
func (v quotaTypeValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config VMModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If quota_type is not set, skip validation (will use default)
	if config.QuotaType.IsNull() || config.QuotaType.IsUnknown() {
		return
	}

	quotaType := config.QuotaType.ValueString()

	// Validate quick_burn requires time_to_live
	if quotaType == "quick_burn" {
		if config.TimeToLive.IsNull() || config.TimeToLive.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				path.Root("time_to_live"),
				"Missing Required Attribute",
				"When quota_type is 'quick_burn', time_to_live must be specified.",
			)
		}
	}

	// Validate product_group requires product_group_id (unless using provider default)
	if quotaType == "product_group" {
		if config.ProductGroupID.IsNull() || config.ProductGroupID.IsUnknown() {
			// This is actually OK if the provider has a default product_group_id
			// We can't check that here, so we'll just warn
			tflog.Debug(ctx, "product_group_id not set, will use provider default if available")
		}
	}
}
