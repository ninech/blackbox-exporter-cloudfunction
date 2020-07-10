variable "project" {
  description = "GCP project to deploy the function to."
}

variable "region" {
  description = "GCP region where the cloud function should run."
}

variable "bucket_name" {
  description = "GCS bucket to deploy the function to."
}

variable "config" {
  description = "Blackbox exporter config as a string."
}

variable "suffix" {
  description = "Suffix to add to the function name. Useful for when delpoying the function multiple times in a project."
  default     = ""
}

variable "ingress_settings" {
  description = "Controls what traffic can reach the function. Allowed values are ALLOW_ALL and ALLOW_INTERNAL_ONLY."
  default     = "ALLOW_INTERNAL_ONLY"
}

variable "create_bucket" {
  description = "Create a GCS bucket with var.bucket_name for this function. If false, var.bucket_name has to be preexisting."
  default     = true
}

variable "available_memory_mb" {
  description = "Memory available to the Cloud Functions instance in MB. Also dictates the amount of CPU power the function gets."
  default     = 256
}
