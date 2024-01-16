data "archive_file" "code" {
  type        = "zip"
  output_path = "${path.module}/blackbox-exporter.zip"
  source {
    filename = "exporter.go"
    content  = file("${path.module}/../exporter.go")
  }
  source {
    filename = "go.mod"
    content  = file("${path.module}/../go.mod")
  }
  source {
    filename = "go.sum"
    content  = file("${path.module}/../go.sum")
  }
  source {
    filename = "config.yml"
    content  = var.config
  }
}

resource "google_storage_bucket" "cloudfunctions" {
  count         = var.create_bucket ? 1 : 0
  name          = var.bucket_name
  project       = var.project
  location      = var.region
  force_destroy = true
}

resource "google_storage_bucket_object" "blackbox_exporter" {
  name       = "cloudfunctions${var.suffix}/${format("blackbox-exporter-%s.zip", data.archive_file.code.output_md5)}"
  bucket     = var.bucket_name
  source     = data.archive_file.code.output_path
  depends_on = [google_storage_bucket.cloudfunctions]
}

resource "google_cloudfunctions_function" "blackbox_exporter" {
  provider    = google-beta
  project     = var.project
  name        = "blackbox-exporter${var.suffix}"
  description = "blackbox exporter as a function"
  runtime     = var.runtime
  region      = var.region

  ingress_settings      = var.ingress_settings
  available_memory_mb   = var.available_memory_mb
  source_archive_bucket = var.bucket_name
  source_archive_object = google_storage_bucket_object.blackbox_exporter.name
  trigger_http          = true
  entry_point           = "Handler"
  timeout               = 10

  timeouts {
    create = 15
    update = 15
    delete = 15
  }
}

# IAM entry for all users to invoke the function
resource "google_cloudfunctions_function_iam_member" "invoker" {
  provider       = google-beta
  project        = google_cloudfunctions_function.blackbox_exporter.project
  region         = google_cloudfunctions_function.blackbox_exporter.region
  cloud_function = google_cloudfunctions_function.blackbox_exporter.name

  role   = "roles/cloudfunctions.invoker"
  member = "allUsers"
}
