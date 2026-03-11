# blackbox-exporter-cloudfunction

This is a [blackbox_exporter](https://github.com/prometheus/blackbox_exporter) that runs on GCP Cloud Functions, insipired by [blackbox-exporter-lambda](https://github.com/discordianfish/blackbox-exporter-lambda).

It can be configured like a normal blackbox_exporter, an example config can be found [here](https://github.com/prometheus/blackbox_exporter/blob/v0.17.0/example.yml).

## Deploying

The `terraform` directory provides a ready to be used Terraform module to easily deploy this function. Just put your blackbox exporter config next to your Terraform config and reference it with the `file` function.

```terraform
module "blackbox-exporter-cloudrun" {
  source      = "github.com/ninech/blackbox-exporter-cloudfunction//terraform?ref=v0.1.2"
  project     = "some-project-id"
  region      = "europe-west6"
  bucket_name = "my-cloudfunctions"
  config      = file("config.yml")
}
```

## Upgrading to v0.2.0

Starting with v0.2.0 this module deploys a Gen2 Cloud Function (backed by Cloud Run) and requires Go 1.22+. Gen2 is **not** a drop-in replacement for Gen1. The following breaking changes apply:

- **Terraform state migration**: The resource type changed from `google_cloudfunctions_function` to `google_cloudfunctions2_function`. Run `terraform state mv` before applying, or Terraform will destroy the old function and create a new one.
- **Service URL changes**: Gen2 URLs follow the Cloud Run format (`https://FUNCTION-HASH-REGION.a.run.app`) instead of the Gen1 format (`https://REGION-PROJECT.cloudfunctions.net/FUNCTION`). Update your Prometheus scrape configs accordingly.
- **IAM role**: The invoker role changed from `roles/cloudfunctions.invoker` to `roles/run.invoker`. Any manually created IAM bindings outside this module need updating.
- **`google-beta` provider**: No longer required. Ensure the `google` provider is configured in your root module.

## Resources

To get consistent performance it is recommended to at set `var.available_memory_mb` to at least `256`. The function does not need that much memory but this will also give it more [CPU power](https://cloud.google.com/functions/pricing#compute_time). The costs should pretty much equalize or be even less as the function will finish way faster than with just 128MB/200Mhz.

This does not require any authentication but is only available within the VPC using the `ALLOW_INTERNAL_ONLY` ingress setting.

## Testing the function

```bash
curl "https://<url_from_output>?target=https://example.org"
```
