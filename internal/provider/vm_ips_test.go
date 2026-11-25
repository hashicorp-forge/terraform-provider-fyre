// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp-forge/terraform-provider-fyre/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestMapVMIPsToList(t *testing.T) {
	ctx := context.Background()

	t.Run("nil vmDetails returns null list", func(t *testing.T) {
		list, diags := mapVMIPsToList(nil)
		require.False(t, diags.HasError())
		require.True(t, list.IsNull())
		require.Equal(t, ipObjectType, list.ElementType(ctx))
	})

	t.Run("nil Ips slice returns null list", func(t *testing.T) {
		vmDetails := &client.VMDetails{
			Ips: nil,
		}
		list, diags := mapVMIPsToList(vmDetails)
		require.False(t, diags.HasError())
		require.True(t, list.IsNull())
		require.Equal(t, ipObjectType, list.ElementType(ctx))
	})

	t.Run("empty Ips slice returns null list", func(t *testing.T) {
		emptyList := []struct {
			Ip    *string `json:"ip,omitempty"`
			Scope *string `json:"scope,omitempty"`
			Type  *string `json:"type,omitempty"`
		}{}
		vmDetails := &client.VMDetails{
			Ips: &emptyList,
		}
		list, diags := mapVMIPsToList(vmDetails)
		require.False(t, diags.HasError())
		require.True(t, list.IsNull())
		require.Equal(t, ipObjectType, list.ElementType(ctx))
	})

	t.Run("populated Ips with full attributes", func(t *testing.T) {
		ip1 := "10.0.0.1"
		type1 := "ipv4"
		scope1 := "private"

		ip2 := "192.168.1.1"
		type2 := "ipv4"
		scope2 := "public"

		ips := []struct {
			Ip    *string `json:"ip,omitempty"`
			Scope *string `json:"scope,omitempty"`
			Type  *string `json:"type,omitempty"`
		}{
			{
				Ip:    &ip1,
				Type:  &type1,
				Scope: &scope1,
			},
			{
				Ip:    &ip2,
				Type:  &type2,
				Scope: &scope2,
			},
		}

		vmDetails := &client.VMDetails{
			Ips: &ips,
		}

		list, diags := mapVMIPsToList(vmDetails)
		require.False(t, diags.HasError())
		require.False(t, list.IsNull())
		require.Len(t, list.Elements(), 2)

		elems := list.Elements()
		obj1, ok := elems[0].(types.Object)
		require.True(t, ok)
		require.Equal(t, types.StringValue("10.0.0.1"), obj1.Attributes()["ip"])
		require.Equal(t, types.StringValue("ipv4"), obj1.Attributes()["type"])
		require.Equal(t, types.StringValue("private"), obj1.Attributes()["scope"])

		obj2, ok := elems[1].(types.Object)
		require.True(t, ok)
		require.Equal(t, types.StringValue("192.168.1.1"), obj2.Attributes()["ip"])
		require.Equal(t, types.StringValue("ipv4"), obj2.Attributes()["type"])
		require.Equal(t, types.StringValue("public"), obj2.Attributes()["scope"])
	})

	t.Run("populated Ips with partial attributes", func(t *testing.T) {
		ip := "10.0.0.2"
		ips := []struct {
			Ip    *string `json:"ip,omitempty"`
			Scope *string `json:"scope,omitempty"`
			Type  *string `json:"type,omitempty"`
		}{
			{
				Ip:    &ip,
				Type:  nil,
				Scope: nil,
			},
		}

		vmDetails := &client.VMDetails{
			Ips: &ips,
		}

		list, diags := mapVMIPsToList(vmDetails)
		require.False(t, diags.HasError())
		require.False(t, list.IsNull())
		require.Len(t, list.Elements(), 1)

		obj, ok := list.Elements()[0].(types.Object)
		require.True(t, ok)
		require.Equal(t, types.StringValue("10.0.0.2"), obj.Attributes()["ip"])
		require.True(t, obj.Attributes()["type"].IsNull())
		require.True(t, obj.Attributes()["scope"].IsNull())
	})
}
