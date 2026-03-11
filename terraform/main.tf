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

resource "google_cloudfunctions2_function" "blackbox_exporter" {
  name        = "blackbox-exporter${var.suffix}"
  location    = var.region
  project     = var.project
  description = "blackbox exporter as a function"

  build_config {
    runtime     = var.runtime
    entry_point = "Handler"
    source {
      storage_source {
        bucket = var.bucket_name
        object = google_storage_bucket_object.blackbox_exporter.name
      }
    }
  }

  service_config {
    available_memory = "${var.available_memory_mb}M"
    timeout_seconds  = 10
    ingress_settings = var.ingress_settings
  }

  timeouts {
    create = "15m"
    update = "15m"
    delete = "15m"
  }
}

# IAM entry for all users to invoke the function
resource "google_cloud_run_service_iam_member" "invoker" {
  project  = google_cloudfunctions2_function.blackbox_exporter.project
  location = google_cloudfunctions2_function.blackbox_exporter.location
  service  = google_cloudfunctions2_function.blackbox_exporter.name

  role   = "roles/run.invoker"
  member = "allUsers"
}
