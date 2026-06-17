package hosting

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/novapanel/novapanel/internal/config"
	"github.com/novapanel/novapanel/internal/db"
	"github.com/novapanel/novapanel/internal/system"
)

type NginxManager struct {
	cfg *config.Config
}

func NewNginxManager(cfg *config.Config) *NginxManager {
	return &NginxManager{cfg: cfg}
}

func (n *NginxManager) CreateVHost(domain *db.Domain) error {
	vhost := n.generateVHost(domain)
	vhostPath := filepath.Join(n.cfg.System.NginxSitesDir, domain.DomainName+".conf")

	if err := system.WriteFile(vhostPath, vhost); err != nil {
		return fmt.Errorf("failed to write vhost config: %w", err)
	}

	result := system.ServiceAction("nginx", "reload")
	if !result.Success {
		return fmt.Errorf("failed to reload nginx: %s", result.Stderr)
	}

	return nil
}

func (n *NginxManager) RemoveVHost(domain *db.Domain) {
	vhostPath := filepath.Join(n.cfg.System.NginxSitesDir, domain.DomainName+".conf")
	system.Execute("rm", "-f", vhostPath)
	system.ServiceAction("nginx", "reload")
}

func (n *NginxManager) EnableSSL(domain *db.Domain) {
	vhostPath := filepath.Join(n.cfg.System.NginxSitesDir, domain.DomainName+".conf")

	sslBlock := fmt.Sprintf(`
    listen 443 ssl http2;
    ssl_certificate %s/%s/fullchain.pem;
    ssl_certificate_key %s/%s/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
`, n.cfg.System.SSLCertDir, domain.DomainName, n.cfg.System.SSLCertDir, domain.DomainName)

	current := system.Execute("cat", vhostPath)
	if current.Success {
		content := current.Stdout
		if !strings.Contains(content, "listen 443") {
			content = strings.Replace(content, "listen 80;", "listen 80;\n\treturn 301 https://$host$request_uri;", 1)
			content = strings.Replace(content, "server_name", sslBlock+"\n\tserver_name", 1)
			system.WriteFile(vhostPath, content)
		}
	}

	system.ServiceAction("nginx", "reload")
}

func (n *NginxManager) UpdatePHP(domain *db.Domain, phpVersion string) {
	vhostPath := filepath.Join(n.cfg.System.NginxSitesDir, domain.DomainName+".conf")
	current := system.Execute("cat", vhostPath)
	if current.Success {
		content := current.Stdout
		oldSocket := fmt.Sprintf("php%d-fpm.sock", 0)
		newSocket := fmt.Sprintf("php%s-fpm.sock", strings.ReplaceAll(phpVersion, ".", ""))
		content = strings.ReplaceAll(content, oldSocket, newSocket)
		system.WriteFile(vhostPath, content)
	}
	system.ServiceAction("nginx", "reload")
}

func (n *NginxManager) SetRedirect(domain *db.Domain, url string, redirectType int) {
	vhostPath := filepath.Join(n.cfg.System.NginxSitesDir, domain.DomainName+".conf")
	current := system.Execute("cat", vhostPath)
	if current.Success {
		content := current.Stdout
		redirectBlock := fmt.Sprintf(`
    return %d %s;
`, redirectType, url)

		lines := strings.Split(content, "\n")
		var newLines []string
		inLocation := false
		for _, line := range lines {
			if strings.Contains(line, "location /") {
				inLocation = true
				newLines = append(newLines, line)
				newLines = append(newLines, redirectBlock)
				continue
			}
			if inLocation && strings.Contains(line, "}") {
				inLocation = false
				continue
			}
			if !inLocation {
				newLines = append(newLines, line)
			}
		}

		system.WriteFile(vhostPath, strings.Join(newLines, "\n"))
		system.ServiceAction("nginx", "reload")
	}
}

func (n *NginxManager) generateVHost(domain *db.Domain) string {
	phpSocket := fmt.Sprintf("php%s-fpm.sock", strings.ReplaceAll(domain.PHPVersion, ".", ""))

	return fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    server_name %s www.%s;

    root %s;
    index index.php index.html index.htm;

    access_log /var/log/nginx/%s.access.log;
    error_log /var/log/nginx/%s.error.log;

    location / {
        try_files $uri $uri/ /index.php?$args;
    }

    location ~ \.php$ {
        fastcgi_pass unix:/var/run/php/%s-fpm.sock;
        fastcgi_index index.php;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }

    location ~ /\.ht {
        deny all;
    }

    location = /favicon.ico {
        log_not_found off;
        access_log off;
    }

    location = /robots.txt {
        log_not_found off;
        access_log off;
    }

    location ~* \.(css|gif|ico|jpeg|jpg|js|png|svg|webp|woff|woff2|ttf|eot)$ {
        expires max;
        log_not_found off;
    }

    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    gzip on;
    gzip_vary on;
    gzip_proxied any;
    gzip_comp_level 6;
    gzip_types text/plain text/css text/xml application/json application/javascript application/xml+rss application/atom+xml image/svg+xml;
}
`, domain.DomainName, domain.DomainName, domain.DocumentRoot,
		domain.DomainName, domain.DomainName, phpSocket)
}
