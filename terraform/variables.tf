variable "project" {
  description = "GCP project to deploy the function to."
  type        = string
}

variable "region" {
  description = "GCP region where the cloud function should run."
  type        = string
}

variable "bucket_name" {
  description = "GCS bucket to deploy the function to."
  type        = string
}

variable "config" {
  description = "Blackbox exporter config as a string."
  type        = string
}

variable "suffix" {
  description = "Suffix to add to the function name. Useful for when deploying the function multiple times in a project."
  type        = string
  default     = ""
}

variable "ingress_settings" {
  description = "Controls what traffic can reach the function. Allowed values are ALLOW_ALL, ALLOW_INTERNAL_ONLY and ALLOW_INTERNAL_AND_GCLB."
  type        = string
  default     = "ALLOW_INTERNAL_ONLY"
}

variable "create_bucket" {
  description = "Create a GCS bucket with var.bucket_name for this function. If false, var.bucket_name has to be preexisting."
  type        = bool
  default     = true
}

variable "available_memory_mb" {
  description = "Memory available to the Cloud Functions instance in MB. Also dictates the amount of CPU power the function gets."
  type        = number
  default     = 256
}

variable "runtime" {
  description = "The runtime which the cloudfunction should use. Check https://cloud.google.com/functions/docs/concepts/execution-environment for possible values."
  type        = string
  default     = "go125"
}
