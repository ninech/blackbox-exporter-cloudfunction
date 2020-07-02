# blackbox-exporter-cloudfunction

This is a [blackbox-exporter](https://github.com/prometheus/blackbox_exporter) that runs on GCP Cloud Functions, insipired by [blackbox-exporter-lambda](https://github.com/discordianfish/blackbox-exporter-lambda).

This does not require any authentication by default but is only available within the VPC using the `ALLOW_INTERNAL_ONLY` ingress setting.

## Deploying

The `terraform` directory provides a ready to be used Terraform module to easily deploy this function.

```terraform
module "blackbox-exporter-cloudrun" {
  source      = "./terraform"
  project     = "some-project-id"
  region      = "europe-west6"
  bucket_name = "my-cloudfunctions"
}
```

## Testing the function

```bash
curl https://<url_from_output>/blackbox-exporter/http --data-urlencode 'target=https://support.nine.ch' --data-urlencode 'config={"preferred_ip_protocol": "ip4"}'
```
