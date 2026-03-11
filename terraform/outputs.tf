output "blackbox_exporter_url" {
  description = "URL of the blackbox exporter service"
  value       = google_cloudfunctions2_function.blackbox_exporter.service_config[0].uri
}
