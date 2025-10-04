terraform {
  required_version = ">= 1.5.7"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 4.0.0"
    }
  }
}

module "iam" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role"
  version = "5.30.0"
}
