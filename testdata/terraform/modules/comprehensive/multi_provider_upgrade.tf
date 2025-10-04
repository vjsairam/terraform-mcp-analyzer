# Comprehensive Multi-Provider and Multi-Module Upgrade Test Case
# Tests complex scenarios with multiple providers and modules upgrading simultaneously

terraform {
  required_version = ">= 1.5.7"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.67" # Old version
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 2.99" # Old version  
    }
    google = {
      source  = "hashicorp/google"
      version = "~> 4.84" # Old version
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.20" # Old version
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.9" # Old version
    }
  }
}

# AWS Provider Configuration
provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "tfug-test"
      Environment = "dev"
    }
  }
}

# Azure Provider Configuration  
provider "azurerm" {
  features {}
}

# Google Provider Configuration
provider "google" {
  project = var.gcp_project_id
  region  = var.gcp_region
  zone    = var.gcp_zone
}

# Kubernetes Provider (will be configured by EKS module)
provider "kubernetes" {
  host                   = module.aws_eks.cluster_endpoint
  cluster_ca_certificate = base64decode(module.aws_eks.cluster_certificate_authority_data)
  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    command     = "aws"
    args        = ["eks", "get-token", "--cluster-name", module.aws_eks.cluster_name]
  }
}

# Helm Provider
provider "helm" {
  kubernetes {
    host                   = module.aws_eks.cluster_endpoint
    cluster_ca_certificate = base64decode(module.aws_eks.cluster_certificate_authority_data)
    exec {
      api_version = "client.authentication.k8s.io/v1beta1"
      command     = "aws"
      args        = ["eks", "get-token", "--cluster-name", module.aws_eks.cluster_name]
    }
  }
}

# =============================================================================
# AWS MODULES (Multiple versions with breaking changes)
# =============================================================================

# VPC Module - v4.x to v5.x upgrade
module "aws_vpc" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role"
  version = "4.0.2" # Old version with breaking changes in v5.x

  name = "${var.project_name}-vpc"
  cidr = var.vpc_cidr

  azs             = data.aws_availability_zones.available.names
  private_subnets = var.private_subnets
  public_subnets  = var.public_subnets

  # These variable names changed in v5.x
  enable_nat_gateway = true  # Renamed to create_nat_gateway
  enable_vpn_gateway = false # Renamed to create_vpn_gateway

  # This was removed in v5.x
  assign_generated_ipv6_cidr_block = false

  # Default behavior changed
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Terraform = "true"
  }
}

# IAM Module - v5.x to v6.x upgrade
module "aws_iam_role" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role"
  version = "5.30.0" # Old version with breaking changes in v6.x

  # Module structure changed - should use //modules/iam-role in v6.x


  # This variable was removed in v6.x
  trusted_role_arns = [
    "arn:aws:iam::${data.aws_caller_identity.current.account_id}:root"
  ]

  role_policy_arns = [
    "arn:aws:iam::aws:policy/ReadOnlyAccess"
  ]

  tags = {
    Project = var.project_name
  }
  create      =      # Variable names that changed in v6.x
  create_role = true # Renamed to 'create'

  name = role_name = "${var.project_name}-role" # Renamed to 'name'

}

# EKS Module - v18.x to v19.x upgrade  
module "aws_eks" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role"
  version = "18.31.2" # Old version with breaking changes in v19.x

  cluster_name    = "${var.project_name}-eks"
  cluster_version = "1.24"

  vpc_id     = module.aws_vpc.vpc_id
  subnet_ids = module.aws_vpc.private_subnets

  # Node group configuration that changed in v19.x
  eks_managed_node_groups = {
    main = {
      desired_size = 2
      max_size     = 4
      min_size     = 1

      instance_types = ["t3.medium"]

      # This configuration changed in v19.x
      k8s_labels = {
        Environment = "dev"
        Terraform   = "true"
      }

      # These fields were restructured in v19.x
      remote_access = {
        ec2_ssh_key = var.key_name
      }
    }
  }

  # Fargate profile configuration changed
  fargate_profiles = {
    default = {
      name = "default"
      selectors = [
        {
          namespace = "kube-system"
          labels = {
            k8s-app = "kube-dns"
          }
        }
      ]
    }
  }

  # IRSA configuration changed in v19.x
  enable_irsa = true

  tags = {
    Terraform = "true"
  }
}

# RDS Module - v5.x to v6.x upgrade
module "aws_rds" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role"
  version = "5.9.0" # Old version with breaking changes in v6.x

  identifier = "${var.project_name}-postgres"

  engine            = "postgres"
  engine_version    = "13.13"
  instance_class    = "db.t3.micro"
  allocated_storage = 20

  db_name  = "tfugtest"
  username = "tfuguser"
  password = var.db_password

  vpc_security_group_ids = [aws_security_group.rds.id]
  db_subnet_group_name   = module.aws_vpc.database_subnet_group

  # Backup configuration that changed in v6.x
  backup_retention_period = 7
  backup_window           = "07:00-09:00"
  maintenance_window      = "Mon:00:00-Mon:03:00"

  # This parameter changed in v6.x
  skip_final_snapshot = true
  deletion_protection = false

  # Parameter group configuration changed
  create_db_parameter_group = true
  parameter_group_name      = "${var.project_name}-postgres-params"
  family                    = "postgres13"

  parameters = [
    {
      name  = "log_connections"
      value = "1"
    }
  ]

  tags = {
    Project = var.project_name
  }
}

# =============================================================================
# AZURE MODULES  
# =============================================================================

# Azure Network Module - version with breaking changes
module "azure_network" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role"
  version = "3.5.0" # Version with potential breaking changes

  resource_group_name = azurerm_resource_group.example.name
  location            = azurerm_resource_group.example.location

  vnet_name     = "${var.project_name}-vnet"
  address_space = "10.1.0.0/16"

  subnet_prefixes = ["10.1.1.0/24", "10.1.2.0/24"]
  subnet_names    = ["subnet1", "subnet2"]

  tags = {
    Environment = "dev"
    Project     = var.project_name
  }

  depends_on = [azurerm_resource_group.example]
}

# Azure AKS Module
module "azure_aks" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role"
  version = "6.8.0" # Version that may have breaking changes

  resource_group_name = azurerm_resource_group.example.name
  location            = azurerm_resource_group.example.location

  cluster_name       = "${var.project_name}-aks"
  kubernetes_version = "1.24.9"

  vnet_subnet_id = module.azure_network.vnet_subnets[0]

  # Node pool configuration
  agents_count = 2
  agents_size  = "Standard_B2s"

  # This might change between versions
  enable_rbac = true

  tags = {
    Environment = "dev"
    Project     = var.project_name
  }
}

# =============================================================================
# GOOGLE CLOUD MODULES
# =============================================================================

# GKE Module - version with breaking changes
module "gcp_gke" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role"
  version = "24.1.0" # Version that may have breaking changes

  project_id = var.gcp_project_id
  name       = "${var.project_name}-gke"
  region     = var.gcp_region
  zones      = ["${var.gcp_zone}"]

  network           = "default"
  subnetwork        = "default"
  ip_range_pods     = ""
  ip_range_services = ""

  # Node pool configuration that might change
  node_pools = [
    {
      name         = "default-node-pool"
      machine_type = "e2-medium"
      min_count    = 1
      max_count    = 3
      disk_size_gb = 100
      disk_type    = "pd-standard"
      image_type   = "COS_CONTAINERD"
      auto_repair  = true
      auto_upgrade = true
      preemptible  = false
    }
  ]

  node_pools_oauth_scopes = {
    all = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]
  }

  node_pools_labels = {
    all = {
      project = var.project_name
    }
  }

  node_pools_tags = {
    all = ["gke-node", "${var.project_name}-gke"]
  }
}

# =============================================================================
# SUPPORTING RESOURCES
# =============================================================================

# AWS Resources
data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_caller_identity" "current" {}

resource "aws_security_group" "rds" {
  name_prefix = "${var.project_name}-rds"
  vpc_id      = module.aws_vpc.vpc_id

  ingress {
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    cidr_blocks = [var.vpc_cidr]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.project_name}-rds-sg"
  }
}

# Azure Resources
resource "azurerm_resource_group" "example" {
  name     = "${var.project_name}-rg"
  location = var.azure_location
}

# =============================================================================
# KUBERNETES/HELM DEPLOYMENTS (using multiple providers)
# =============================================================================

# Kubernetes namespace
resource "kubernetes_namespace" "app" {
  metadata {
    name = "${var.project_name}-app"
  }

  depends_on = [module.aws_eks]
}

# Helm chart deployment that might have version compatibility issues
resource "helm_release" "nginx_ingress" {
  name       = "nginx-ingress"
  repository = "https://kubernetes.github.io/ingress-nginx"
  chart      = "ingress-nginx"
  version    = "4.0.18" # Old version that might have breaking changes

  namespace        = kubernetes_namespace.app.metadata[0].name
  create_namespace = false

  # Values that might change between chart versions
  values = [
    yamlencode({
      controller = {
        service = {
          type = "LoadBalancer"
        }
        metrics = {
          enabled = true
        }
        # This configuration might change between versions
        replicaCount = 2
        resources = {
          requests = {
            cpu    = "100m"
            memory = "128Mi"
          }
        }
      }
    })
  ]

  depends_on = [module.aws_eks]
}

# =============================================================================
# VARIABLES
# =============================================================================

variable "project_name" {
  description = "Name of the project"
  type        = string
  default     = "tfug-comprehensive-test"
}

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-west-2"
}

variable "azure_location" {
  description = "Azure location"
  type        = string
  default     = "West Europe"
}

variable "gcp_project_id" {
  description = "GCP project ID"
  type        = string
  default     = "example-project-id"
}

variable "gcp_region" {
  description = "GCP region"
  type        = string
  default     = "us-central1"
}

variable "gcp_zone" {
  description = "GCP zone"
  type        = string
  default     = "us-central1-a"
}

variable "vpc_cidr" {
  description = "CIDR block for VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "private_subnets" {
  description = "Private subnet CIDRs"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
}

variable "public_subnets" {
  description = "Public subnet CIDRs"
  type        = list(string)
  default     = ["10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"]
}

variable "key_name" {
  description = "AWS Key Pair name"
  type        = string
  default     = "example-key"
}

variable "db_password" {
  description = "RDS password"
  type        = string
  default     = "examplepassword123"
  sensitive   = true
}