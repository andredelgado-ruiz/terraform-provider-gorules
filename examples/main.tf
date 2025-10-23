# Ejemplo de configuración para usar el GoRules Terraform Provider

terraform {
  required_version = ">= 1.0"
  
  required_providers {
    gorules = {
      source  = "andredelgadoruiz/gorules"
      version = "~> 0.1.0"
    }
  }
}

# Configuración del provider GoRules
provider "gorules" {
  # URL base de tu instancia de GoRules
  base_url = "https://tu-instancia.gorules.io"
  
  # Token de acceso personal (PAT)
  # Es recomendable usar variables de entorno: TF_VAR_gorules_token
  token = var.gorules_token
}

# Variable para el token (define esto en terraform.tfvars o como variable de entorno)
variable "gorules_token" {
  description = "Personal Access Token para GoRules API"
  type        = string
  sensitive   = true
}

# Ejemplo: Crear un proyecto
resource "gorules_project" "example" {
  name = "Mi Proyecto de Prueba"
  key  = "mi-proyecto-prueba"  # Debe seguir el patrón: ^[a-z0-9]{2,}(-[a-z0-9]+)*$
}

# Ejemplo: Crear un entorno en el proyecto
resource "gorules_environment" "development" {
  project_id = gorules_project.example.id
  name       = "development"
  type       = "development"  # Requerido según el schema
}

# Ejemplo: Crear un grupo
resource "gorules_group" "developers" {
  name        = "Developers"
  project_id  = gorules_project.example.id
  permissions = ["read", "write"]  # Lista de permisos
}

# Outputs útiles
output "project_id" {
  description = "ID del proyecto creado"
  value       = gorules_project.example.id
}

output "environment_id" {
  description = "ID del entorno creado"
  value       = gorules_environment.development.id
}