package organization

import (
	"context"

	"github.com/aiven/aiven-go-client"
	"github.com/aiven/terraform-provider-aiven/internal/schemautil"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func DatasourceOrganization() *schema.Resource {
	return &schema.Resource{
		ReadContext: datasourceOrganizationRead,
		Description: "The Organization data source provides information about the existing Aiven Organization.",
		Schema:      schemautil.ResourceSchemaAsDatasourceSchema(aivenOrganizationSchema, "name"),
	}
}

func datasourceOrganizationRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*aiven.Client)

	name := d.Get("name").(string)

	r, err := client.Accounts.List()
	if err != nil {
		return diag.FromErr(err)
	}

	for _, ac := range r.Accounts {
		if ac.Name == name {
			d.SetId(ac.Id)
			return resourceOrganizationRead(ctx, d, m)
		}
	}

	return diag.Errorf("organization %s not found", name)
}
