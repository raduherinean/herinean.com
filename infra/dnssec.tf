resource "cloudflare_zone_dnssec" "d" {
  for_each = var.zones
  zone_id  = each.value
  status   = "active"
}

output "ds_records" {
  description = "Paste each DS record at that domain's registrar. Format: keytag algorithm digest_type digest"
  value       = { for z, r in cloudflare_zone_dnssec.d : var.domains[z] => r.ds }
}
