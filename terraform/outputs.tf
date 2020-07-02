output "blackbox_exporter_url" {
  description = "URL of the blackbox exporter service"
  value       = google_cloudfunctions_function.blackbox_exporter.https_trigger_url
}
