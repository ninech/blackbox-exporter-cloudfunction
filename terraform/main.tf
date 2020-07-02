data "archive_file" "code" {
  type        = "zip"
  source_dir  = "${path.module}/../blackbox-exporter"
  output_path = "${path.module}/blackbox-exporter.zip"
}

resource "google_storage_bucket" "cloudfunctions" {
  count         = var.create_bucket ? 1 : 0
  name          = var.bucket_name
  project       = var.project
  location      = var.region
  force_destroy = true
}

resource "google_storage_bucket_object" "blackbox_exporter" {
  name       = "cloudfunctions/${format("blackbox-exporter-%s.zip", data.archive_file.code.output_md5)}"
  bucket     = var.bucket_name
  source     = data.archive_file.code.output_path
  depends_on = [google_storage_bucket.cloudfunctions]
}

resource "google_cloudfunctions_function" "blackbox_exporter" {
  provider    = google-beta
  project     = var.project
  name        = "blackbox-exporter"
  description = "blackbox exporter as a function"
  runtime     = "go113"
  region      = var.region

  ingress_settings      = "ALLOW_INTERNAL_ONLY"
  available_memory_mb   = 128
  source_archive_bucket = var.bucket_name
  source_archive_object = google_storage_bucket_object.blackbox_exporter.name
  trigger_http          = true
  entry_point           = "Handler"
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
