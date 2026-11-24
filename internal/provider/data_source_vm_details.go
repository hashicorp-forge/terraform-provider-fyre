// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp-forge/terraform-provider-fyre/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &DataSourceVMDetails{}

// NewDataSourceVMDetails creates a new instance of the VM details data source.
// This factory function is called by the provider to instantiate the data source.
func NewDataSourceVMDetails() datasource.DataSource {
	return &DataSourceVMDetails{}
}

type DataSourceVMDetails struct {
	client      *client.ClientWithResponses
	defaultSite string
}

type VMDetailsModel struct {
	ID                  types.String `tfsdk:"id"`
	VMID                types.String `tfsdk:"vm_id"`
	IP                  types.String `tfsdk:"ip"`
	FQDN                types.String `tfsdk:"fqdn"`
	Site                types.String `tfsdk:"site"`
	Location            types.String `tfsdk:"location"`
	Hostname            types.String `tfsdk:"hostname"`
	Domain              types.String `tfsdk:"domain"`
	Description         types.String `tfsdk:"description"`
	State               types.String `tfsdk:"state"`
	Platform            types.String `tfsdk:"platform"`
	OS                  types.String `tfsdk:"os"`
	CPU                 types.Int64  `tfsdk:"cpu"`
	Memory              types.Int64  `tfsdk:"memory"`
	OSDisk              types.Int64  `tfsdk:"os_disk"`
	DiskDriver          types.String `tfsdk:"disk_driver"`
	QuotaType           types.String `tfsdk:"quota_type"`
	ProductGroupID      types.Int64  `tfsdk:"product_group_id"`
	ProductGroup        types.String `tfsdk:"product_group"`
	Pingable            types.String `tfsdk:"pingable"`
	PingableLastChecked types.String `tfsdk:"pingable_last_checked"`
	Sshable             types.String `tfsdk:"sshable"`
	SshableLastChecked  types.String `tfsdk:"sshable_last_checked"`
	TransferComment     types.String `tfsdk:"transfer_comment"`
	Comment             types.String `tfsdk:"comment"`
	Expiration          types.String `tfsdk:"expiration"`
	Timezone            types.String `tfsdk:"timezone"`
	SecurityLock        types.String `tfsdk:"security_lock"`
	Compliance          types.String `tfsdk:"compliance"`
	DisableDelete       types.String `tfsdk:"disable_delete"`
	AutoPatch           types.String `tfsdk:"auto_patch"`
	Created             types.String `tfsdk:"created"`
	CreatedISO8601      types.String `tfsdk:"created_iso8601"`
	HostDown            types.String `tfsdk:"host_down"`
	AllowFloatingIP     types.String `tfsdk:"allow_floating_ip"`
	User                types.Object `tfsdk:"user"`
	IPs                 types.List   `tfsdk:"ips"`
}

// Metadata sets the data source type name for the VM details data source.
// The type name is used in Terraform configurations as "fyre_vm_details".
func (d *DataSourceVMDetails) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm_details"
}

// Schema defines the structure and attributes of the VM details data source.
// It specifies optional vm_id, ip, and fqdn parameters (at least one required),
// optional site parameter, and extensive computed attributes including VM configuration,
// resource allocation, networking, owner information, and operational status.
func (d *DataSourceVMDetails) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches detailed information about a Fyre VM. You must provide at least one identifier: vm_id, ip, or fqdn. The data source will try each non-null identifier until it successfully retrieves the VM details.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The identifier used to retrieve the VM (same as the successful lookup field)",
				Computed:            true,
			},
			"vm_id": schema.StringAttribute{
				MarkdownDescription: "VM identifier (format: x-xxxxxxx). At least one of vm_id, ip, or fqdn must be provided.",
				Optional:            true,
			},
			"ip": schema.StringAttribute{
				MarkdownDescription: "IP address of the VM. At least one of vm_id, ip, or fqdn must be provided.",
				Optional:            true,
			},
			"fqdn": schema.StringAttribute{
				MarkdownDescription: "Fully Qualified Domain Name of the VM (must be in DNS). At least one of vm_id, ip, or fqdn must be provided.",
				Optional:            true,
			},
			"site": schema.StringAttribute{
				MarkdownDescription: "Site location (svl or rtp). Defaults to 'svl' or inherits from provider configuration.",
				Optional:            true,
				Computed:            true,
			},
			"location": schema.StringAttribute{
				MarkdownDescription: "VM location",
				Computed:            true,
			},
			"hostname": schema.StringAttribute{
				MarkdownDescription: "VM hostname",
				Computed:            true,
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "VM domain",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "VM description",
				Computed:            true,
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "Current state of the VM (e.g., running, stopped)",
				Computed:            true,
			},
			"platform": schema.StringAttribute{
				MarkdownDescription: "Platform type (x, pvm, or z)",
				Computed:            true,
			},
			"os": schema.StringAttribute{
				MarkdownDescription: "Operating system",
				Computed:            true,
			},
			"cpu": schema.Int64Attribute{
				MarkdownDescription: "Number of CPUs allocated",
				Computed:            true,
			},
			"memory": schema.Int64Attribute{
				MarkdownDescription: "Memory allocated in GB",
				Computed:            true,
			},
			"os_disk": schema.Int64Attribute{
				MarkdownDescription: "OS disk size in GB",
				Computed:            true,
			},
			"disk_driver": schema.StringAttribute{
				MarkdownDescription: "Disk driver type",
				Computed:            true,
			},
			"quota_type": schema.StringAttribute{
				MarkdownDescription: "Quota type (product_group or quick_burn)",
				Computed:            true,
			},
			"product_group_id": schema.Int64Attribute{
				MarkdownDescription: "Product group ID",
				Computed:            true,
			},
			"product_group": schema.StringAttribute{
				MarkdownDescription: "Product group name",
				Computed:            true,
			},
			"pingable": schema.StringAttribute{
				MarkdownDescription: "Whether the VM is pingable (y/n)",
				Computed:            true,
			},
			"pingable_last_checked": schema.StringAttribute{
				MarkdownDescription: "Last time pingable status was checked",
				Computed:            true,
			},
			"sshable": schema.StringAttribute{
				MarkdownDescription: "Whether the VM is SSH accessible (y/n)",
				Computed:            true,
			},
			"sshable_last_checked": schema.StringAttribute{
				MarkdownDescription: "Last time SSH accessibility was checked",
				Computed:            true,
			},
			"transfer_comment": schema.StringAttribute{
				MarkdownDescription: "Comment for VM transfer",
				Computed:            true,
			},
			"comment": schema.StringAttribute{
				MarkdownDescription: "VM comment",
				Computed:            true,
			},
			"expiration": schema.StringAttribute{
				MarkdownDescription: "VM expiration time",
				Computed:            true,
			},
			"timezone": schema.StringAttribute{
				MarkdownDescription: "VM timezone",
				Computed:            true,
			},
			"security_lock": schema.StringAttribute{
				MarkdownDescription: "Security lock status",
				Computed:            true,
			},
			"compliance": schema.StringAttribute{
				MarkdownDescription: "Compliance status",
				Computed:            true,
			},
			"disable_delete": schema.StringAttribute{
				MarkdownDescription: "Whether deletion is disabled (y/n)",
				Computed:            true,
			},
			"auto_patch": schema.StringAttribute{
				MarkdownDescription: "Auto-patch status",
				Computed:            true,
			},
			"created": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp",
				Computed:            true,
			},
			"created_iso8601": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp in ISO 8601 format",
				Computed:            true,
			},
			"host_down": schema.StringAttribute{
				MarkdownDescription: "Whether the host is down (y/n)",
				Computed:            true,
			},
			"allow_floating_ip": schema.StringAttribute{
				MarkdownDescription: "Whether floating IPs are allowed (y/n)",
				Computed:            true,
			},
			"user": schema.SingleNestedAttribute{
				MarkdownDescription: "VM owner information",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"id": schema.Int64Attribute{
						MarkdownDescription: "User ID",
						Computed:            true,
					},
					"username": schema.StringAttribute{
						MarkdownDescription: "Username",
						Computed:            true,
					},
					"email": schema.StringAttribute{
						MarkdownDescription: "Email address",
						Computed:            true,
					},
					"display_name": schema.StringAttribute{
						MarkdownDescription: "Display name",
						Computed:            true,
					},
					"status": schema.StringAttribute{
						MarkdownDescription: "User status",
						Computed:            true,
					},
				},
			},
			"ips": schema.ListNestedAttribute{
				MarkdownDescription: "List of IP addresses assigned to the VM",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"ip": schema.StringAttribute{
							MarkdownDescription: "IP address",
							Computed:            true,
						},
						"type": schema.StringAttribute{
							MarkdownDescription: "IP type (public or private)",
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

// Configure initializes the VM details data source with the Fyre API client
// and default site configuration from the provider. This method is called by the
// framework during provider initialization.
func (d *DataSourceVMDetails) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(*FyreProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *FyreProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = providerData.Client
	d.defaultSite = providerData.DefaultSite
}

// Read retrieves comprehensive details about a Fyre VM from the API. It accepts vm_id,
// ip, or fqdn as identifiers (at least one required) and tries each in order until
// successful. Returns extensive VM information including configuration, resources,
// networking, owner details, and operational status.
func (d *DataSourceVMDetails) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data VMDetailsModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate that at least one identifier is provided
	if data.VMID.IsNull() && data.IP.IsNull() && data.FQDN.IsNull() {
		resp.Diagnostics.AddError(
			"Missing Required Attribute",
			"At least one of vm_id, ip, or fqdn must be provided",
		)
		return
	}

	// Prepare site parameter
	site := client.GetVMDetailsParamsSite(data.Site.ValueString())
	if site == "" {
		site = client.GetVMDetailsParamsSite(d.defaultSite)
	}

	// Try each identifier in order until one succeeds
	identifiers := []struct {
		value string
		name  string
	}{
		{data.VMID.ValueString(), "vm_id"},
		{data.IP.ValueString(), "ip"},
		{data.FQDN.ValueString(), "fqdn"},
	}

	var vmResp *client.GetVMDetailsResponse
	var err error
	var successfulIdentifier string

	for _, identifier := range identifiers {
		if identifier.value == "" {
			continue
		}

		tflog.Debug(ctx, fmt.Sprintf("Trying to fetch VM details using %s: %s", identifier.name, identifier.value))

		vmResp, err = d.client.GetVMDetailsWithResponse(ctx, identifier.value, &client.GetVMDetailsParams{
			Site: &site,
		})
		if err != nil {
			tflog.Debug(ctx, fmt.Sprintf("Error fetching VM with %s: %s", identifier.name, err))
			continue
		}

		if vmResp.StatusCode() == 200 && vmResp.JSON200 != nil {
			successfulIdentifier = identifier.value
			tflog.Debug(ctx, fmt.Sprintf("Successfully fetched VM details using %s", identifier.name))
			break
		}

		tflog.Debug(ctx, fmt.Sprintf("Failed to fetch VM with %s, status: %d", identifier.name, vmResp.StatusCode()))
	}

	if successfulIdentifier == "" {
		resp.Diagnostics.AddError(
			"VM Not Found",
			fmt.Sprintf("Unable to find VM using any of the provided identifiers. Last error: %v", err),
		)
		return
	}

	if vmResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Parse Error",
			fmt.Sprintf("Unable to parse VM response. Body: %s", string(vmResp.Body)),
		)
		return
	}

	// Map response to Terraform state
	vmDetails := vmResp.JSON200

	data.ID = types.StringValue(successfulIdentifier)
	data.Site = types.StringValue(string(site))

	// Map all fields from VMDetails
	if vmDetails.VmId != nil {
		data.VMID = types.StringValue(*vmDetails.VmId)
	}

	if vmDetails.Location != nil {
		data.Location = types.StringValue(*vmDetails.Location)
	} else {
		data.Location = types.StringNull()
	}

	if vmDetails.Hostname != nil {
		data.Hostname = types.StringValue(*vmDetails.Hostname)
	} else {
		data.Hostname = types.StringNull()
	}

	if vmDetails.Domain != nil {
		data.Domain = types.StringValue(*vmDetails.Domain)
	} else {
		data.Domain = types.StringNull()
	}

	if vmDetails.Description != nil {
		data.Description = types.StringValue(*vmDetails.Description)
	} else {
		data.Description = types.StringNull()
	}

	if vmDetails.State != nil {
		data.State = types.StringValue(*vmDetails.State)
	} else {
		data.State = types.StringNull()
	}

	if vmDetails.Platform != nil {
		data.Platform = types.StringValue(*vmDetails.Platform)
	} else {
		data.Platform = types.StringNull()
	}

	if vmDetails.Os != nil {
		data.OS = types.StringValue(*vmDetails.Os)
	} else {
		data.OS = types.StringNull()
	}

	if vmDetails.Cpu != nil {
		data.CPU = types.Int64Value(int64(*vmDetails.Cpu))
	} else {
		data.CPU = types.Int64Null()
	}

	if vmDetails.Memory != nil {
		data.Memory = types.Int64Value(int64(*vmDetails.Memory))
	} else {
		data.Memory = types.Int64Null()
	}

	if vmDetails.OsDisk != nil {
		data.OSDisk = types.Int64Value(int64(*vmDetails.OsDisk))
	} else {
		data.OSDisk = types.Int64Null()
	}

	if vmDetails.DiskDriver != nil {
		data.DiskDriver = types.StringValue(*vmDetails.DiskDriver)
	} else {
		data.DiskDriver = types.StringNull()
	}

	if vmDetails.QuotaType != nil {
		data.QuotaType = types.StringValue(*vmDetails.QuotaType)
	} else {
		data.QuotaType = types.StringNull()
	}

	if vmDetails.ProductGroupId != nil {
		data.ProductGroupID = types.Int64Value(int64(*vmDetails.ProductGroupId))
	} else {
		data.ProductGroupID = types.Int64Null()
	}

	if vmDetails.ProductGroup != nil {
		data.ProductGroup = types.StringValue(*vmDetails.ProductGroup)
	} else {
		data.ProductGroup = types.StringNull()
	}

	if vmDetails.Fqdn != nil {
		data.FQDN = types.StringValue(*vmDetails.Fqdn)
	}

	if vmDetails.Pingable != nil {
		data.Pingable = types.StringValue(*vmDetails.Pingable)
	} else {
		data.Pingable = types.StringNull()
	}

	if vmDetails.PingableLastChecked != nil {
		data.PingableLastChecked = types.StringValue(*vmDetails.PingableLastChecked)
	} else {
		data.PingableLastChecked = types.StringNull()
	}

	if vmDetails.Sshable != nil {
		data.Sshable = types.StringValue(*vmDetails.Sshable)
	} else {
		data.Sshable = types.StringNull()
	}

	if vmDetails.SshableLastChecked != nil {
		data.SshableLastChecked = types.StringValue(*vmDetails.SshableLastChecked)
	} else {
		data.SshableLastChecked = types.StringNull()
	}

	if vmDetails.TransferComment != nil {
		data.TransferComment = types.StringValue(*vmDetails.TransferComment)
	} else {
		data.TransferComment = types.StringNull()
	}

	if vmDetails.Comment != nil {
		data.Comment = types.StringValue(*vmDetails.Comment)
	} else {
		data.Comment = types.StringNull()
	}

	if vmDetails.Expiration != nil {
		data.Expiration = types.StringValue(*vmDetails.Expiration)
	} else {
		data.Expiration = types.StringNull()
	}

	if vmDetails.Timezone != nil {
		data.Timezone = types.StringValue(*vmDetails.Timezone)
	} else {
		data.Timezone = types.StringNull()
	}

	if vmDetails.SecurityLock != nil {
		data.SecurityLock = types.StringValue(*vmDetails.SecurityLock)
	} else {
		data.SecurityLock = types.StringNull()
	}

	if vmDetails.Compliance != nil {
		data.Compliance = types.StringValue(*vmDetails.Compliance)
	} else {
		data.Compliance = types.StringNull()
	}

	if vmDetails.DisableDelete != nil {
		data.DisableDelete = types.StringValue(*vmDetails.DisableDelete)
	} else {
		data.DisableDelete = types.StringNull()
	}

	if vmDetails.AutoPatch != nil {
		data.AutoPatch = types.StringValue(*vmDetails.AutoPatch)
	} else {
		data.AutoPatch = types.StringNull()
	}

	if vmDetails.Created != nil {
		data.Created = types.StringValue(*vmDetails.Created)
	} else {
		data.Created = types.StringNull()
	}

	if vmDetails.CreatedIso8601 != nil {
		data.CreatedISO8601 = types.StringValue(*vmDetails.CreatedIso8601)
	} else {
		data.CreatedISO8601 = types.StringNull()
	}

	if vmDetails.HostDown != nil {
		data.HostDown = types.StringValue(*vmDetails.HostDown)
	} else {
		data.HostDown = types.StringNull()
	}

	if vmDetails.AllowFloatingIp != nil {
		data.AllowFloatingIP = types.StringValue(*vmDetails.AllowFloatingIp)
	} else {
		data.AllowFloatingIP = types.StringNull()
	}

	// Map user object
	userAttrTypes := map[string]attr.Type{
		"id":           types.Int64Type,
		"username":     types.StringType,
		"email":        types.StringType,
		"display_name": types.StringType,
		"status":       types.StringType,
	}

	if vmDetails.User != nil {
		userAttrs := map[string]attr.Value{
			"id":           types.Int64Null(),
			"username":     types.StringNull(),
			"email":        types.StringNull(),
			"display_name": types.StringNull(),
			"status":       types.StringNull(),
		}

		if vmDetails.User.Id != nil {
			userAttrs["id"] = types.Int64Value(int64(*vmDetails.User.Id))
		}
		if vmDetails.User.Username != nil {
			userAttrs["username"] = types.StringValue(*vmDetails.User.Username)
		}
		if vmDetails.User.Email != nil {
			userAttrs["email"] = types.StringValue(*vmDetails.User.Email)
		}
		if vmDetails.User.DisplayName != nil {
			userAttrs["display_name"] = types.StringValue(*vmDetails.User.DisplayName)
		}
		if vmDetails.User.Status != nil {
			userAttrs["status"] = types.StringValue(*vmDetails.User.Status)
		}

		userObj, diags := types.ObjectValue(userAttrTypes, userAttrs)
		resp.Diagnostics.Append(diags...)
		if !resp.Diagnostics.HasError() {
			data.User = userObj
		}
	} else {
		data.User = types.ObjectNull(userAttrTypes)
	}

	// Map IPs list
	ipsList, diags := mapVMIPsToList(vmDetails)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.IPs = ipsList

	// Extract first IP if available for the ip field
	if vmDetails.Ips != nil && len(*vmDetails.Ips) > 0 {
		if (*vmDetails.Ips)[0].Ip != nil {
			data.IP = types.StringValue(*(*vmDetails.Ips)[0].Ip)
		}
	}

	tflog.Trace(ctx, "read vm_details data source")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
