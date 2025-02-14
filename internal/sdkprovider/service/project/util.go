package project

import (
	"github.com/aiven/aiven-go-client"
	"github.com/aiven/terraform-provider-aiven/internal/schemautil"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// accountIDPointer returns the account ID pointer to use in a request to the Aiven API.
// This is limited to the domain of the billing groups and projects.
// If the owner_entity_id is set, it will be used as the account ID. Otherwise, the account_id will be used.
func accountIDPointer(client *aiven.Client, d *schema.ResourceData) (*string, error) {
	var accountID *string

	ownerEntityID, ok := d.GetOk("owner_entity_id")
	if ok {
		ownerEntityID, err := schemautil.NormalizeOrganizationID(client, ownerEntityID.(string))
		if err != nil {
			return nil, err
		}

		if len(ownerEntityID) == 0 {
			return nil, nil
		}

		accountID = &ownerEntityID
	} else {
		// TODO: Remove this when account_id is removed.
		accountID = schemautil.OptionalStringPointer(d, "account_id")
	}

	return accountID, nil
}
