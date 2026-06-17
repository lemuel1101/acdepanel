package hosting

import (
	"fmt"

	"github.com/novapanel/novapanel/internal/config"
	"github.com/novapanel/novapanel/internal/system"
)

type SSLManager struct {
	cfg *config.Config
}

func NewSSLManager(cfg *config.Config) *SSLManager {
	return &SSLManager{cfg: cfg}
}

func (s *SSLManager) Obtain(domain string) error {
	if !system.FileExists("/usr/bin/certbot") {
		return fmt.Errorf("certbot is not installed")
	}

	result := system.Execute("certbot", "--nginx", "-d", domain, "-d", "www."+domain,
		"--non-interactive", "--agree-tos", "--email", "admin@"+domain,
		"--redirect")

	if !result.Success {
		return fmt.Errorf("certbot failed: %s", result.Stderr)
	}

	return nil
}

func (s *SSLManager) RenewAll() error {
	result := system.Execute("certbot", "renew", "--non-interactive")
	if !result.Success {
		return fmt.Errorf("certbot renew failed: %s", result.Stderr)
	}
	return nil
}

func (s *SSLManager) CheckStatus(domain string) bool {
	certPath := fmt.Sprintf("%s/%s/fullchain.pem", s.cfg.System.SSLCertDir, domain)
	return system.FileExists(certPath)
}
