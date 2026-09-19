locals {
  # Free-plan zone settings. Mirage/Polish are Pro+, Auto Minify no longer exists: nothing to disable there.
  zone_settings = {
    always_use_https         = "on"
    min_tls_version          = "1.3"
    tls_1_3                  = "on"
    http3                    = "on"
    "0rtt"                   = "off"
    ipv6                     = "on"
    brotli                   = "on"
    opportunistic_encryption = "on"
    ssl                      = "strict"
    browser_check            = "on"
    # injectors — every one of these adds a script or a resource to the page
    email_obfuscation = "off"
    rocket_loader     = "off"
    fonts             = "off"
    speed_brain       = "off"
  }
  zone_setting_pairs = merge([
    for z, id in var.zones : {
      for s, v in local.zone_settings : "${z}.${s}" => { zone_id = id, setting = s, value = v }
    }
  ]...)
}

resource "cloudflare_zone_setting" "s" {
  for_each   = local.zone_setting_pairs
  zone_id    = each.value.zone_id
  setting_id = each.value.setting
  value      = each.value.value
}

resource "cloudflare_zone_setting" "hsts" {
  for_each   = var.zones
  zone_id    = each.value
  setting_id = "security_header"
  value = {
    strict_transport_security = {
      enabled            = true
      max_age            = 63072000
      include_subdomains = true
      preload            = true
      nosniff            = true
    }
  }
}

resource "cloudflare_bot_management" "b" {
  for_each   = var.zones
  zone_id    = each.value
  fight_mode = false
}
