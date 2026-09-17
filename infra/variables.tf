variable "account_id" {
  type        = string
  description = "Cloudflare account id (TF_VAR_account_id from scripts/env.sh)"
}

variable "zones" {
  type        = map(string)
  description = "short name -> zone id"
}

variable "domains" {
  type = map(string)
  default = {
    com  = "herinean.com"
    ro   = "herinean.ro"
    net  = "herinean.net"
    info = "herinean.info"
  }
}

variable "redirect_zones" {
  type    = set(string)
  default = ["ro", "net", "info"]
}

variable "mx" {
  type        = map(number)
  description = "Google Workspace MX hosts -> priority, identical on all four zones"
  default     = { "smtp.google.com" = 1 }
}

variable "dmarc_cf_rua" {
  type        = string
  description = "Cloudflare DMARC Management report address already in _dmarc.herinean.com (mailto:<id>@dmarc-reports.cloudflare.net)"
}

variable "dkim_txt" {
  type        = string
  description = "google._domainkey TXT value in the API's quoted form (public key; not secret)"
}

variable "dmarc_policy" {
  type    = string
  default = "none" # flipped to "reject" in Task 6 step 8
}

variable "mta_sts_id" {
  type    = string
  default = "20260917T000000Z" # bump whenever the policy file changes
}
