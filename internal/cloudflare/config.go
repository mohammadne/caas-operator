package cloudflare

type Config struct {
	CloudflareURL string `koanf:"cloudflare_url"`
	Token         string `koanf:"token"`
	ZoneID        string `koanf:"zone_id"`
}
