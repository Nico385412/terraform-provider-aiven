---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "aiven_flink_application Data Source - terraform-provider-aiven"
subcategory: ""
description: |-
  The Flink Application data source provides information about the existing Aiven Flink Application.
---

# aiven_flink_application (Data Source)

The Flink Application data source provides information about the existing Aiven Flink Application.

## Example Usage

```terraform
data "aiven_flink_application" "app1" {
  project      = data.aiven_project.pr1.project
  service_name = "<SERVICE_NAME>"
  name         = "<APPLICATION_NAME>"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) Application name
- `project` (String) Identifies the project this resource belongs to. To set up proper dependencies please refer to this variable as a reference. This property cannot be changed, doing so forces recreation of the resource.
- `service_name` (String) Specifies the name of the service that this resource belongs to. To set up proper dependencies please refer to this variable as a reference. This property cannot be changed, doing so forces recreation of the resource.

### Read-Only

- `application_id` (String) Application ID
- `created_at` (String) Application creation time
- `created_by` (String) Application creator
- `id` (String) The ID of this resource.
- `updated_at` (String) Application update time
- `updated_by` (String) Application updater
