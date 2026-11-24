terraform {
  required_providers {
    goaway = {
      source  = "registry.terraform.io/rasty94/goaway"
      version = "~> 0.95"
    }
  }
}

provider "goaway" {
  endpoint = "http://192.168.1.2:8080"
  # api_key = var.goaway_api_key # or set GOAWAY_API_KEY
}

resource "goaway_upstream" "cloudflare" {
  address = "1.1.1.1:53"
}

resource "goaway_resolution" "nas" {
  domain = "nas.lan"
  value  = "192.168.1.50"
  type   = "A"
}

resource "goaway_whitelist_domain" "internal_cdn" {
  domain = "cdn.internal.example.com"
}

resource "goaway_group" "kids" {
  name                = "kids"
  description         = "Restricted browsing profile"
  use_global_policies = true
}
