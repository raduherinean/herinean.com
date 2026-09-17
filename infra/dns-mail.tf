# All four domains are Google Workspace domains of one mailbox (.net primary, the rest aliases). Every one keeps its MX.
# Everything else is hardened: strict SPF, DMARC reject with strict alignment; DKIM, MTA-STS and TLS-RPT on .com (the sending domain).
# DKIM records on .net and .info pre-exist and stay unmanaged.

locals {
  mx_pairs = merge([
    for z, id in var.zones : {
      for host, prio in var.mx : "${z}.${host}" => { zone_id = id, name = var.domains[z], host = host, prio = prio }
    }
  ]...)
  # Each zone reports to its own Cloudflare DMARC Management address and to dmarc@ on itself (same inbox via the alias).
  dmarc_rua = { for z, d in var.domains : z => "${var.dmarc_cf_rua[z]},mailto:dmarc@${d}" }
}

resource "cloudflare_dns_record" "mx" {
  for_each = local.mx_pairs
  zone_id  = each.value.zone_id
  name     = each.value.name
  type     = "MX"
  content  = each.value.host
  priority = each.value.prio
  ttl      = 1
}

resource "cloudflare_dns_record" "spf" {
  for_each = var.zones
  zone_id  = each.value
  name     = var.domains[each.key]
  type     = "TXT"
  content  = "\"v=spf1 include:_spf.google.com -all\""
  ttl      = 1
}

resource "cloudflare_dns_record" "dmarc" {
  for_each = var.zones
  zone_id  = each.value
  name     = "_dmarc.${var.domains[each.key]}"
  type     = "TXT"
  # .com (sending domain) and .net (Workspace primary) go p=none → verified by a signed test message → reject (var.dmarc_policy);
  # .ro and .info are reject from the start: nothing sends as them.
  content  = "\"v=DMARC1; p=${contains(["com", "net"], each.key) ? var.dmarc_policy : "reject"}; adkim=s; aspf=s; rua=${local.dmarc_rua[each.key]}\""
  ttl      = 1
}

resource "cloudflare_dns_record" "com_dkim" {
  zone_id = var.zones["com"]
  name    = "google._domainkey.${var.domains["com"]}"
  type    = "TXT"
  content = var.dkim_txt # already in the API's quoted form
  ttl     = 1
}

resource "cloudflare_dns_record" "com_mta_sts" {
  zone_id = var.zones["com"]
  name    = "_mta-sts.${var.domains["com"]}"
  type    = "TXT"
  content = "\"v=STSv1; id=${var.mta_sts_id}\""
  ttl     = 1
}

resource "cloudflare_dns_record" "com_tlsrpt" {
  zone_id = var.zones["com"]
  name    = "_smtp._tls.${var.domains["com"]}"
  type    = "TXT"
  content = "\"v=TLSRPTv1; rua=mailto:dmarc@herinean.com\""
  ttl     = 1
}
