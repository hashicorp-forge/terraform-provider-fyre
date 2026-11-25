// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"github.com/hashicorp-forge/terraform-provider-fyre/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var ipAttrTypes = map[string]attr.Type{
	"ip":    types.StringType,
	"type":  types.StringType,
	"scope": types.StringType,
}
var ipObjectType = types.ObjectType{AttrTypes: ipAttrTypes}

func mapVMIPsToList(vmDetails *client.VMDetails) (types.List, diag.Diagnostics) {
	if vmDetails == nil || vmDetails.Ips == nil || len(*vmDetails.Ips) == 0 {
		return types.ListNull(ipObjectType), nil
	}
	ipElements := make([]attr.Value, 0, len(*vmDetails.Ips))

	for _, ip := range *vmDetails.Ips {
		ipAttr := map[string]attr.Value{
			"ip":    types.StringNull(),
			"type":  types.StringNull(),
			"scope": types.StringNull(),
		}
		if ip.Ip != nil {
			ipAttr["ip"] = types.StringValue(*ip.Ip)
		}
		if ip.Type != nil {
			ipAttr["type"] = types.StringValue(*ip.Type)
		}
		if ip.Scope != nil {
			ipAttr["scope"] = types.StringValue(*ip.Scope)
		}
		ipObj, diags := types.ObjectValue(ipAttrTypes, ipAttr)
		if diags.HasError() {
			return types.ListNull(ipObjectType), diags
		}
		ipElements = append(ipElements, ipObj)
	}
	return types.ListValue(ipObjectType, ipElements)
}
