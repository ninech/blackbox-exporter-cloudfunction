variable "project" {
  description = "GCP project to deploy the function to."
}

variable "region" {
  description = "GCP region where the cloud function should run."
}

variable "bucket_name" {
  description = "GCS bucket to deploy the function to."
}

variable "create_bucket" {
  description = "Create a GCS bucket with var.bucket_name for this function. If false, var.bucket_name has to be preexisting."
  default     = true
}
