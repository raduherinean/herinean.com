locals {
  # CAs used by Cloudflare Universal SSL, per Cloudflare's docs; iodef reports to the security alias.
  caa = [
    { tag = "issue", value = "letsencrypt.org" },
    { tag = "issue", value = "pki.goog; cansignhttpexchanges=yes" },
    { tag = "issue", value = "ssl.com" },
    { tag = "issuewild", value = "letsencrypt.org" },
    { tag = "issuewild", value = "pki.goog; cansignhttpexchanges=yes" },
    { tag = "issuewild", value = "ssl.com" },
    { tag = "iodef", value = "mailto:security@herinean.com" },
  ]
  caa_pairs = merge([
    for z, id in var.zones : {
      for i, c in local.caa : "${z}.${i}" => { zone_id = id, name = var.domains[z], tag = c.tag, value = c.value }
    }
  ]...)

  # Hosts that exist only to redirect. Proxied placeholders per Cloudflare's own guidance.
  # The herinean.com apex is absent on purpose: the Workers custom domain creates it.
  web_hosts = merge(
    { for z in var.redirect_zones : "${z}.apex" => { zone = z, name = var.domains[z] } },
    { for z in var.redirect_zones : "${z}.www" => { zone = z, name = "www.${var.domains[z]}" } },
    { "com.www" = { zone = "com", name = "www.herinean.com" } },
  )
}

resource "cloudflare_dns_record" "caa" {
  for_each = local.caa_pairs
  zone_id  = each.value.zone_id
  name     = each.value.name
  type     = "CAA"
  ttl      = 1
  data = {
    flags = 0
    tag   = each.value.tag
    value = each.value.value
  }
}

resource "cloudflare_dns_record" "web_a" {
  for_each = local.web_hosts
  zone_id  = var.zones[each.value.zone]
  name     = each.value.name
  type     = "A"
  content  = "192.0.2.1"
  proxied  = true
  ttl      = 1
}

resource "cloudflare_dns_record" "web_aaaa" {
  for_each = local.web_hosts
  zone_id  = var.zones[each.value.zone]
  name     = each.value.name
  type     = "AAAA"
  content  = "100::"
  proxied  = true
  ttl      = 1
}
