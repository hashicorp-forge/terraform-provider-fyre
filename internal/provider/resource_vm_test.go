// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccResourceVM runs the acceptance test for creating, updating, and
// destroying VM's in Fyre with product_group_id set at resource level.
func TestAccResourceVM(t *testing.T) {
	if os.Getenv("FYRE_USERNAME") == "" || os.Getenv("FYRE_API_KEY") == "" {
		t.Skip("FYRE_USERNAME and FYRE_API_KEY must be set for acceptance tests")
	}

	productGroupID := os.Getenv("FYRE_ACC_PROD_GID")
	if productGroupID == "" {
		t.Skip("FYRE_ACC_PROD_GID must be set for acceptance tests")
	}

	// Our primary validator that tests modifying the the state of the resource
	// many times. Other sub-tests have a smaller scope.
	t.Run("with resource product group id", func(t *testing.T) {
		resource.Test(t, resource.TestCase{
			PreCheck:                 func() { testAccPreCheck(t) },
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				// Create and Read testing
				{
					Config: testAccResourceVMConfigBasic("RedHat 9.6", 2, 4, "Test VM for Terraform acceptance testing", "24", "n", productGroupID),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("fyre_vm.test", "os", "RedHat 9.6"),
						resource.TestCheckResourceAttr("fyre_vm.test", "cpu", "2"),
						resource.TestCheckResourceAttr("fyre_vm.test", "memory", "4"),
						resource.TestCheckResourceAttr("fyre_vm.test", "description", "Test VM for Terraform acceptance testing"),
						resource.TestCheckResourceAttr("fyre_vm.test", "expiration", "24"),
						resource.TestCheckResourceAttr("fyre_vm.test", "disable_delete", "n"),
						resource.TestCheckResourceAttr("fyre_vm.test", "product_group_id", productGroupID),
						resource.TestCheckResourceAttrSet("fyre_vm.test", "id"),
						resource.TestCheckResourceAttrSet("fyre_vm.test", "vm_id"),
						resource.TestCheckResourceAttrSet("fyre_vm.test", "site"),
						resource.TestCheckResourceAttrSet("fyre_vm.test", "platform"),
						resource.TestCheckResourceAttrSet("fyre_vm.test", "expiration_time"),
						resource.TestCheckResourceAttrSet("fyre_vm.test", "ips.#"),
						resource.TestCheckResourceAttrSet("fyre_vm.test", "ips.0.ip"),
						resource.TestCheckResourceAttrSet("fyre_vm.test", "ips.0.type"),
						resource.TestCheckResourceAttrSet("fyre_vm.test", "ips.0.scope"),
					),
				},
				// Verify ips is stable — no diff after create (Computed-only, no user values)
				{
					Config:   testAccResourceVMConfigBasic("RedHat 9.6", 2, 4, "Test VM for Terraform acceptance testing", "24", "n", productGroupID),
					PlanOnly: true,
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttrSet("fyre_vm.test", "ips.#"),
						resource.TestCheckResourceAttrSet("fyre_vm.test", "ips.0.ip"),
					),
				},
				// Update CPU and Memory
				{
					Config: testAccResourceVMConfigBasic("RedHat 9.6", 4, 8, "Test VM for Terraform acceptance testing", "24", "n", productGroupID),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("fyre_vm.test", "cpu", "4"),
						resource.TestCheckResourceAttr("fyre_vm.test", "memory", "8"),
						resource.TestCheckResourceAttr("fyre_vm.test", "description", "Test VM for Terraform acceptance testing"),
						resource.TestCheckResourceAttrSet("fyre_vm.test", "id"),
						resource.TestCheckResourceAttrSet("fyre_vm.test", "vm_id"),
					),
				},
				// Update Description
				{
					Config: testAccResourceVMConfigBasic("RedHat 9.6", 4, 8, "Updated test VM description", "24", "n", productGroupID),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("fyre_vm.test", "cpu", "4"),
						resource.TestCheckResourceAttr("fyre_vm.test", "memory", "8"),
						resource.TestCheckResourceAttr("fyre_vm.test", "description", "Updated test VM description"),
					),
				},
				// Update Expiration
				{
					Config: testAccResourceVMConfigBasic("RedHat 9.6", 4, 8, "Updated test VM description", "48", "n", productGroupID),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("fyre_vm.test", "expiration", "48"),
					),
				},
				// Update Disable Delete
				{
					Config: testAccResourceVMConfigBasic("RedHat 9.6", 4, 8, "Updated test VM description", "48", "y", productGroupID),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("fyre_vm.test", "disable_delete", "y"),
					),
				},
				// Update Password
				{
					Config: testAccResourceVMConfigWithPassword("RedHat 9.6", 4, 8, "Updated test VM description", "48", "y", "NewTestPassword123!", productGroupID),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("fyre_vm.test", "password", "NewTestPassword123!"),
					),
				},
				// Update Additional Disks
				{
					Config: testAccResourceVMConfigWithDisks("RedHat 9.6", 4, 8, "Updated test VM description", "48", "y", productGroupID),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("fyre_vm.test", "additional_disks.#", "2"),
						resource.TestCheckResourceAttr("fyre_vm.test", "additional_disks.0", "50"),
						resource.TestCheckResourceAttr("fyre_vm.test", "additional_disks.1", "100"),
					),
				},
				// ImportState testing
				{
					ResourceName:      "fyre_vm.test",
					ImportState:       true,
					ImportStateVerify: true,
					// Ignore fields that are not returned by the API or are sensitive
					// expiration: user input (relative time) not returned by API, only expiration_time (absolute) is returned
					ImportStateVerifyIgnore: []string{"password", "ssh_keys", "expiration"},
				},
				// Re-enable delete so the test sweeper can actually clean up the resource
				{
					Config: testAccResourceVMConfigWithDisks("RedHat 9.6", 4, 8, "Updated test VM description", "48", "n", productGroupID),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("fyre_vm.test", "disable_delete", "n"),
					),
				},
			},
		})
	})

	t.Run("with provider product group id", func(t *testing.T) {
		resource.Test(t, resource.TestCase{
			PreCheck:                 func() { testAccPreCheck(t) },
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				// Create with provider-level product_group_id
				{
					Config: testAccResourceVMConfigProviderLevel("RedHat 9.6", 2, 4, "Test VM with provider-level product_group_id", "24", "n", productGroupID),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("fyre_vm.test_provider", "os", "RedHat 9.6"),
						resource.TestCheckResourceAttr("fyre_vm.test_provider", "cpu", "2"),
						resource.TestCheckResourceAttr("fyre_vm.test_provider", "memory", "4"),
						resource.TestCheckResourceAttr("fyre_vm.test_provider", "description", "Test VM with provider-level product_group_id"),
						resource.TestCheckResourceAttr("fyre_vm.test_provider", "product_group_id", productGroupID),
						resource.TestCheckResourceAttrSet("fyre_vm.test_provider", "id"),
						resource.TestCheckResourceAttrSet("fyre_vm.test_provider", "vm_id"),
					),
				},
				// Update with provider-level product_group_id still inherited
				{
					Config: testAccResourceVMConfigProviderLevel("RedHat 9.6", 4, 8, "Updated VM with provider-level product_group_id", "24", "n", productGroupID),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("fyre_vm.test_provider", "cpu", "4"),
						resource.TestCheckResourceAttr("fyre_vm.test_provider", "memory", "8"),
						resource.TestCheckResourceAttr("fyre_vm.test_provider", "description", "Updated VM with provider-level product_group_id"),
						resource.TestCheckResourceAttr("fyre_vm.test_provider", "product_group_id", productGroupID),
					),
				},
			},
		})
	})

	t.Run("with missing time_to_live and quick_burn quota_type", func(t *testing.T) {
		resource.Test(t, resource.TestCase{
			PreCheck:                 func() { testAccPreCheck(t) },
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{
					Config: `
	provider "fyre" {
		site = "svl"
	}

	resource "fyre_vm" "test" {
		# Basic properties
		os               = "RedHat 9.6"
		cpu              = 1
		memory           = 2
		description      = "A test VM"
		platform         = "x"
		expiration       = 48
		disable_delete   = "n"

		# Quota type
		quota_type       = "quick_burn"
		# Missing time_to_live - should fail validation
	}
	`,
					ExpectError: regexp.MustCompile(`time_to_live must be specified`),
				},
			},
		})
	})

	t.Run("with time_to_live and quick_burn quota_type", func(t *testing.T) {
		resource.Test(t, resource.TestCase{
			PreCheck:                 func() { testAccPreCheck(t) },
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{
					Config: testAccResourceVMConfigQuickBurn("RedHat 9.6", 2, 4, "Test quick_burn VM", "4", productGroupID),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("fyre_vm.test_qb", "os", "RedHat 9.6"),
						resource.TestCheckResourceAttr("fyre_vm.test_qb", "quota_type", "quick_burn"),
						resource.TestCheckResourceAttr("fyre_vm.test_qb", "time_to_live", "4"),
						resource.TestCheckResourceAttr("fyre_vm.test_qb", "product_group_id", productGroupID),
						resource.TestCheckResourceAttrSet("fyre_vm.test_qb", "id"),
						resource.TestCheckResourceAttrSet("fyre_vm.test_qb", "vm_id"),
					),
				},
			},
		})
	})
}

// TestUnitResourceVM_IPsReadOnly verifies that the ips attribute rejects
// user-supplied values at plan time (no API credentials required).
//
// ips was declared Optional+Computed, allowing users to set IP values
// that the Fyre API silently ignores. This caused "Provider produced
// inconsistent result after apply" and left the resource tainted.
// ips is now Computed-only — the framework rejects the config before
// any API call is made.
func TestUnitResourceVM_IPsReadOnly(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "fyre" {
  site = "svl"
}

resource "fyre_vm" "test_ips" {
  os             = "CentOS Stream 9"
  platform       = "x"
  cpu            = 2
  memory         = 4
  expiration     = "4"
  disable_delete = "n"

  ips = [
    {
      ip    = "192.0.2.1"
      type  = "public"
      scope = "external"
    }
  ]
}
`,
				ExpectError: regexp.MustCompile(`Invalid Configuration for Read-Only Attribute`),
			},
		},
	})
}

func testAccResourceVMConfigBasic(os string, cpu, memory int, description, expiration, disableDelete, productGroupID string) string {
	return fmt.Sprintf(`
provider "fyre" {
  site = "svl"
}

resource "fyre_vm" "test" {
  os               = %[1]q
  cpu              = %[2]d
  memory           = %[3]d
  description      = %[4]q
  platform         = "x"
  expiration       = %[5]q
  disable_delete   = %[6]q
  product_group_id = %[7]q
}
`, os, cpu, memory, description, expiration, disableDelete, productGroupID)
}

func testAccResourceVMConfigWithPassword(os string, cpu, memory int, description, expiration, disableDelete, password, productGroupID string) string {
	return fmt.Sprintf(`
provider "fyre" {
  site = "svl"
}

resource "fyre_vm" "test" {
  os               = %[1]q
  cpu              = %[2]d
  memory           = %[3]d
  description      = %[4]q
  platform         = "x"
  expiration       = %[5]q
  disable_delete   = %[6]q
  password         = %[7]q
  product_group_id = %[8]q
}
`, os, cpu, memory, description, expiration, disableDelete, password, productGroupID)
}

func testAccResourceVMConfigWithDisks(os string, cpu, memory int, description, expiration, disableDelete, productGroupID string) string {
	return fmt.Sprintf(`
provider "fyre" {
  site = "svl"
}

resource "fyre_vm" "test" {
  os               = %[1]q
  cpu              = %[2]d
  memory           = %[3]d
  description      = %[4]q
  platform         = "x"
  expiration       = %[5]q
  disable_delete   = %[6]q
  additional_disks = ["50", "100"]
  product_group_id = %[7]q
}
`, os, cpu, memory, description, expiration, disableDelete, productGroupID)
}

func testAccResourceVMConfigProviderLevel(os string, cpu, memory int, description, expiration, disableDelete, productGroupID string) string {
	return fmt.Sprintf(`
provider "fyre" {
  site = "svl"
  product_group_id = %[7]s
}

resource "fyre_vm" "test_provider" {
  os             = %[1]q
  cpu            = %[2]d
  memory         = %[3]d
  description    = %[4]q
  platform       = "x"
  expiration     = %[5]q
  disable_delete = %[6]q
  # product_group_id inherited from provider
}
`, os, cpu, memory, description, expiration, disableDelete, productGroupID)
}

func testAccResourceVMConfigQuickBurn(os string, cpu, memory int, description, timeToLive, productGroupID string) string {
	return fmt.Sprintf(`
provider "fyre" {
  site = "svl"
}

resource "fyre_vm" "test_qb" {
  os               = %[1]q
  cpu              = %[2]d
  memory           = %[3]d
  description      = %[4]q
  platform         = "x"
  quota_type       = "quick_burn"
  time_to_live     = %[5]q
  product_group_id = %[6]q
}
`, os, cpu, memory, description, timeToLive, productGroupID)
}
