# Google Cloud Provider v4 to v5 Upgrade Test Case
# Tests breaking changes in GCP provider upgrade

terraform {
  required_version = ">= 1.5.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 4.84"  # Old version to trigger upgrade rules
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

# Compute Instance - various deprecations in v5.0
resource "google_compute_instance" "example" {
  name         = "example-instance"
  machine_type = "e2-micro"
  zone         = var.zone

  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-11"
      size  = 20
      type  = "pd-standard"
    }
  }

  # This argument was deprecated in v5.0
  metadata_startup_script = "echo Hello World"

  # Network configuration that changed
  network_interface {
    network = "default"
    
    # This changed in v5.0
    access_config {
      # Ephemeral IP
    }
  }

  # Service account configuration changed
  service_account {
    # This field was deprecated
    email  = var.service_account_email
    scopes = ["cloud-platform"]
  }

  # Metadata block structure changed
  metadata = {
    startup-script = "echo Hello from startup script"
    # This metadata key format changed
    ssh-keys = "user:${file("~/.ssh/id_rsa.pub")}"
  }

  tags = ["http-server", "https-server"]
}

# Cloud SQL Instance - breaking changes in v5.0
resource "google_sql_database_instance" "example" {
  name             = "example-db-instance"
  database_version = "POSTGRES_13"
  region           = var.region

  settings {
    tier = "db-f1-micro"
    
    # This block structure changed significantly
    backup_configuration {
      enabled                        = true
      start_time                     = "02:00"
      point_in_time_recovery_enabled = true
      
      # This field was renamed in v5.0
      binary_log_enabled = true
    }

    # IP configuration changed
    ip_configuration {
      ipv4_enabled    = true
      private_network = google_compute_network.example.id
      
      # This was deprecated
      require_ssl = true
      
      authorized_networks {
        name  = "public"
        value = "0.0.0.0/0"
      }
    }

    # Database flags format changed
    database_flags {
      name  = "log_checkpoints"
      value = "on"
    }

    database_flags {
      name  = "log_connections"  
      value = "on"
    }

    # Maintenance window configuration changed
    maintenance_window {
      day  = 7
      hour = 3
      
      # This field was removed in v5.0
      update_track = "stable"
    }
  }

  # Deletion protection default changed
  deletion_protection = false
}

# VPC Network
resource "google_compute_network" "example" {
  name                    = "example-network"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "example" {
  name          = "example-subnetwork"
  ip_cidr_range = "10.0.1.0/24"
  region        = var.region
  network       = google_compute_network.example.id

  # Secondary ranges format changed in v5.0
  secondary_ip_range {
    range_name    = "services-range"
    ip_cidr_range = "192.168.1.0/24"
  }

  secondary_ip_range {
    range_name    = "pod-ranges"
    ip_cidr_range = "192.168.64.0/22"
  }
}

# GKE Cluster - major changes in v5.0
resource "google_container_cluster" "example" {
  name     = "example-gke-cluster"
  location = var.region

  # We can't create a cluster with no node pool defined, but we want to only use
  # separately managed node pools. So we create the smallest possible default
  # node pool and immediately delete it.
  remove_default_node_pool = true
  initial_node_count       = 1

  # Network configuration
  network    = google_compute_network.example.name
  subnetwork = google_compute_subnetwork.example.name

  # IP allocation policy changed in v5.0
  ip_allocation_policy {
    cluster_secondary_range_name  = "pod-ranges"
    services_secondary_range_name = "services-range"
  }

  # Private cluster configuration changed
  private_cluster_config {
    enable_private_nodes    = true
    enable_private_endpoint = false
    master_ipv4_cidr_block  = "172.16.0.0/28"
  }

  # Master auth configuration changed
  master_auth {
    # This field was deprecated in v5.0
    username = ""
    password = ""

    client_certificate_config {
      issue_client_certificate = false
    }
  }

  # Network policy configuration
  network_policy {
    enabled = true
    
    # This field was removed in v5.0
    provider = "CALICO"
  }

  # Addons configuration structure changed
  addons_config {
    http_load_balancing {
      disabled = false
    }

    horizontal_pod_autoscaling {
      disabled = false
    }

    # This addon configuration changed in v5.0
    network_policy_config {
      disabled = false
    }
  }
}

# GKE Node Pool - configuration changes in v5.0
resource "google_container_node_pool" "example" {
  name       = "example-node-pool"
  location   = var.region
  cluster    = google_container_cluster.example.name
  node_count = 1

  node_config {
    preemptible  = true
    machine_type = "e2-medium"

    # Service account configuration changed
    service_account = var.service_account_email
    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]

    # Metadata changed in v5.0
    metadata = {
      disable-legacy-endpoints = "true"
    }

    # Shielded instance config was restructured
    shielded_instance_config {
      enable_secure_boot          = true
      enable_integrity_monitoring = true
    }

    # Disk configuration changed
    disk_size_gb = 100
    disk_type    = "pd-standard"
    
    # This field was deprecated in v5.0
    local_ssd_count = 0

    tags = ["gke-node", "example-gke-cluster-node"]
  }

  # Autoscaling configuration
  autoscaling {
    min_node_count = 1
    max_node_count = 3
  }

  # Management configuration changed
  management {
    auto_repair  = true
    auto_upgrade = true
  }
}

# Cloud Storage Bucket - uniform bucket access became default in v5.0
resource "google_storage_bucket" "example" {
  name     = "example-bucket-${random_string.bucket_suffix.result}"
  location = var.region

  # This became the default in v5.0
  uniform_bucket_level_access = false

  # Lifecycle rules format changed
  lifecycle_rule {
    condition {
      age = 30
    }
    action {
      type = "Delete"
    }
  }

  # CORS configuration structure changed slightly
  cors {
    origin          = ["http://example.com"]
    method          = ["GET", "HEAD", "PUT", "POST", "DELETE"]
    response_header = ["*"]
    max_age_seconds = 3600
  }

  # Versioning configuration
  versioning {
    enabled = true
  }
}

resource "random_string" "bucket_suffix" {
  length  = 8
  special = false
  upper   = false
}

# Variables
variable "project_id" {
  description = "GCP Project ID"
  type        = string
  default     = "example-project-id"
}

variable "region" {
  description = "GCP Region"
  type        = string
  default     = "us-central1"
}

variable "zone" {
  description = "GCP Zone"
  type        = string
  default     = "us-central1-a"
}

variable "service_account_email" {
  description = "Service Account Email"
  type        = string
  default     = "example@example-project.iam.gserviceaccount.com"
}