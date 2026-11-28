package server

import "github.com/containeroo/appkit/auth/proxy"

// testProxyConfig supplies resolved framework headers and an explicit loopback trust boundary.
func testProxyConfig() proxy.Config {
	cfg := proxy.Defaults()
	cfg.TrustedCIDRs = []string{"127.0.0.1/32"}

	return cfg
}
