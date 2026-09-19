# Everything on the redirect-only zones → https://herinean.com, path and query preserved.
resource "cloudflare_ruleset" "redirect" {
  for_each = var.redirect_zones
  zone_id  = var.zones[each.key]
  name     = "redirect to herinean.com"
  kind     = "zone"
  phase    = "http_request_dynamic_redirect"
  rules = [{
    description = "301 to herinean.com, keep path and query"
    expression  = "true"
    action      = "redirect"
    enabled     = true
    action_parameters = {
      from_value = {
        status_code           = 301
        preserve_query_string = true
        target_url = {
          expression = "concat(\"https://herinean.com\", http.request.uri.path)"
        }
      }
    }
  }]
}

# www.herinean.com → apex. Trailing-slash normalisation is the Worker's job (assets html_handling).
resource "cloudflare_ruleset" "www" {
  zone_id = var.zones["com"]
  name    = "www to apex"
  kind    = "zone"
  phase   = "http_request_dynamic_redirect"
  rules = [{
    description = "301 www to apex, keep path and query"
    expression  = "http.host eq \"www.herinean.com\""
    action      = "redirect"
    enabled     = true
    action_parameters = {
      from_value = {
        status_code           = 301
        preserve_query_string = true
        target_url = {
          expression = "concat(\"https://herinean.com\", http.request.uri.path)"
        }
      }
    }
  }]
}
