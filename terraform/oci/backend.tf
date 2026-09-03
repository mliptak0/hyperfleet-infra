# Remote state on the same GCS backend as the GKE stacks (see
# terraform/README.md), under its own prefix. Configure with:
#   terraform init -backend-config=ci.tfbackend
terraform {
  backend "gcs" {}
}
