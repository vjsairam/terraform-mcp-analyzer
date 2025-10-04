# Azure Provider v2 to v3 Upgrade Test Case
# Tests major breaking changes in Azure provider upgrade

terraform {
  required_version = ">= 1.5.0"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 2.99"  # Old version to trigger upgrade rules
    }
  }
}

provider "azurerm" {
  features {}
}

# Resource Group
resource "azurerm_resource_group" "example" {
  name     = "example-resources"
  location = "West Europe"
}

# Virtual Machine - major breaking change in v3.0
resource "azurerm_virtual_machine" "example" {
  name                = "example-vm"
  location            = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name
  vm_size             = "Standard_B1s"

  # This configuration structure changed significantly in v3.0
  storage_image_reference {
    publisher = "Canonical"
    offer     = "UbuntuServer" 
    sku       = "18.04-LTS"
    version   = "latest"
  }

  storage_os_disk {
    name          = "example-osdisk"
    caching       = "ReadWrite"
    create_option = "FromImage"
  }

  # Boot diagnostics default behavior changed
  boot_diagnostics {
    enabled = false  # This became default enabled in v3.0
  }

  # Network interface
  network_interface_ids = [
    azurerm_network_interface.example.id,
  ]

  # OS Profile - structure changed
  os_profile {
    computer_name  = "hostname"
    admin_username = "testadmin"
  }

  os_profile_linux_config {
    disable_password_authentication = false
  }

  # These were deprecated
  delete_os_disk_on_termination    = true
  delete_data_disks_on_termination = true
}

# Network Interface
resource "azurerm_network_interface" "example" {
  name                = "example-nic"
  location            = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name

  ip_configuration {
    name                          = "internal"
    subnet_id                     = azurerm_subnet.example.id
    private_ip_address_allocation = "Dynamic"
  }
}

# Virtual Network
resource "azurerm_virtual_network" "example" {
  name                = "example-network"
  address_space       = ["10.0.0.0/16"]
  location            = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name
}

resource "azurerm_subnet" "example" {
  name                 = "internal"
  resource_group_name  = azurerm_resource_group.example.name
  virtual_network_name = azurerm_virtual_network.example.name
  address_prefixes     = ["10.0.2.0/24"]
}

# Storage Account - deprecated arguments
resource "azurerm_storage_account" "example" {
  name                     = "examplestorageaccount"
  resource_group_name      = azurerm_resource_group.example.name
  location                 = azurerm_resource_group.example.location
  account_tier             = "Standard"
  account_replication_type = "LRS"

  # This argument was deprecated in v3.0
  enable_advanced_threat_protection = false
}

# Kubernetes Cluster - significant changes in v3.0
resource "azurerm_kubernetes_cluster" "example" {
  name                = "example-aks1"
  location            = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name
  dns_prefix          = "exampleaks1"

  # This block was renamed to default_node_pool in v3.0
  agent_pool_profile {
    name           = "default"
    count          = 1
    vm_size        = "Standard_D2_v2"
    os_type        = "Linux"
    os_disk_size_gb = 30
  }

  service_principal {
    client_id     = var.client_id
    client_secret = var.client_secret
  }

  tags = {
    Environment = "Development"
  }
}

# App Service Plan - changed in v3.0  
resource "azurerm_app_service_plan" "example" {
  name                = "example-appserviceplan"
  location            = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name

  sku {
    tier = "Standard"
    size = "S1"
  }
}

# Variables for the configuration
variable "client_id" {
  description = "Azure Service Principal Client ID"
  type        = string
  default     = "example-client-id"
}

variable "client_secret" {
  description = "Azure Service Principal Client Secret"  
  type        = string
  default     = "example-client-secret"
  sensitive   = true
}