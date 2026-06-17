#!/usr/bin/env bash
set -euo pipefail

NOVAPANEL_REPO="https://github.com/lemuel1101/acdepanel"
NOVAPANEL_CONFIG_DIR="/etc/novapanel"
NOVAPANEL_DATA_DIR="/var/lib/novapanel"
NOVAPANEL_LOG_DIR="/var/log/novapanel"
NOVAPANEL_BACKUP_DIR="/var/backups/novapanel"
NOVAPANEL_BINARY="/usr/local/bin/novapanel"
NOVAPANEL_CLI="/usr/local/bin/novactl"
NOVAPANEL_FRONTEND_DIR="${NOVAPANEL_DATA_DIR}/frontend"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log()    { echo -e "${GREEN}[✓]${NC} $1"; }
warn()   { echo -e "${YELLOW}[!]${NC} $1"; }
error()  { echo -e "${RED}[✗]${NC} $1"; exit 1; }
info()   { echo -e "${CYAN}[i]${NC} $1"; }

banner() {
    echo ""
    echo -e "${CYAN}  _   _  ___  __     ____  _          _    _ "
    echo -e " | \ | |/ _ \ \ \   / /  \/ | |   / \  | |  "
    echo -e " |  \| | | | | \ \ / /| |\/| | |  / _ \ | |  "
    echo -e " | |\  | |_| |  \ V / | |  | | | / ___ \| |  "
    echo -e " |_| \_|\___/    \_/  |_|  |_| |_/   \_|_|  ${NC}"
    echo -e "${GREEN}  Server Control Panel - One-Click Installer${NC}"
    echo ""
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "This script must be run as root"
    fi
}

detect_os() {
    if [[ ! -f /etc/os-release ]]; then
        error "Cannot detect OS. This installer supports Ubuntu only."
    fi

    source /etc/os-release
    if [[ "$ID" != "ubuntu" ]]; then
        error "Unsupported OS: $ID. This installer supports Ubuntu only."
    fi

    log "Detected Ubuntu $VERSION_ID ($VERSION_CODENAME)"
}

setup_environment() {
    info "Setting up environment..."
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq
    log "Package repository updated"
}

install_dependencies() {
    info "Installing dependencies..."
    apt-get install -y -qq \
        curl wget git nginx certbot python3-certbot-nginx \
        mysql-server php-fpm php-cli php-mysql php-curl php-json \
        php-mbstring php-xml php-zip php-gd php-bcmath \
        postfix dovecot-core dovecot-imapd dovecot-pop3d \
        ufw fail2ban unzip tar gzip cron systemd
    log "Dependencies installed"
}

install_novapanel() {
    info "Installing NovaPanel..."
    mkdir -p "${NOVAPANEL_CONFIG_DIR}" "${NOVAPANEL_DATA_DIR}" "${NOVAPANEL_LOG_DIR}" "${NOVAPANEL_BACKUP_DIR}" "${NOVAPANEL_FRONTEND_DIR}"

    if command -v go &> /dev/null; then
        tmp_dir=$(mktemp -d)
        cd "${tmp_dir}"
        git clone --depth 1 "${NOVAPANEL_REPO}.git" .
        go build -o "${NOVAPANEL_BINARY}" ./cmd/novapanel/
        go build -o "${NOVAPANEL_CLI}" ./cmd/novactl/
        rm -rf "${tmp_dir}"
    else
        wget -q https://go.dev/dl/go1.21.5.linux-amd64.tar.gz -O /tmp/go.tar.gz
        tar -C /usr/local -xzf /tmp/go.tar.gz
        export PATH=$PATH:/usr/local/go/bin
        tmp_dir=$(mktemp -d)
        cd "${tmp_dir}"
        git clone --depth 1 "${NOVAPANEL_REPO}.git" .
        /usr/local/go/bin/go build -o "${NOVAPANEL_BINARY}" ./cmd/novapanel/
        /usr/local/go/bin/go build -o "${NOVAPANEL_CLI}" ./cmd/novactl/
        rm -rf "${tmp_dir}"
    fi

    chmod +x "${NOVAPANEL_BINARY}" "${NOVAPANEL_CLI}"
    log "NovaPanel binaries installed"
}

setup_config() {
    info "Configuring NovaPanel..."
    if [[ ! -f "${NOVAPANEL_CONFIG_DIR}/config.yaml" ]]; then
        cat > "${NOVAPANEL_CONFIG_DIR}/config.yaml" << 'EOF'
server:
  host: "0.0.0.0"
  port: 8080
  panel_port: 2083
  mode: "release"
  log_level: "info"
  frontend_dir: "/var/lib/novapanel/frontend"

database:
  driver: "sqlite"
  host: "localhost"
  port: 5432
  user: "novapanel"
  password: ""
  name: "novapanel"
  ssl_mode: "disable"

jwt:
  secret: "__GENERATED_SECRET__"
  access_token_ttl: 15
  refresh_token_ttl: 4320

system:
  nginx_path: "/etc/nginx"
  nginx_sites_dir: "/etc/nginx/sites-enabled"
  php_base_dir: "/etc/php"
  home_dir_prefix: "/home"
  ssl_cert_dir: "/etc/letsencrypt/live"
  logs_dir: "/var/log/novapanel"
  data_dir: "/var/lib/novapanel"

backup:
  dir: "/var/backups/novapanel"
  retention_days: 30
  s3_bucket: ""
  s3_region: ""
  s3_access_key: ""
  s3_secret_key: ""

installed: true
EOF
        if command -v openssl &> /dev/null; then
            SECRET=$(openssl rand -hex 32)
            sed -i "s/__GENERATED_SECRET__/${SECRET}/g" "${NOVAPANEL_CONFIG_DIR}/config.yaml"
        fi
    fi
    log "Configuration created at ${NOVAPANEL_CONFIG_DIR}/config.yaml"
}

setup_systemd() {
    info "Setting up systemd service..."
    cat > /etc/systemd/system/novapanel.service << 'SERVICEEOF'
[Unit]
Description=NovaPanel - Server Control Panel
After=network.target nginx.service mysql.service
Wants=network.target

[Service]
Type=simple
User=root
Group=root
ExecStart=/usr/local/bin/novapanel
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=10
Environment=NOVAPANEL_CONFIG=/etc/novapanel/config.yaml
Environment=GIN_MODE=release
StandardOutput=journal
StandardError=journal
LimitNOFILE=65536
LimitNPROC=65536
ProtectSystem=full
ProtectHome=false
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
SERVICEEOF

    systemctl daemon-reload
    systemctl enable novapanel
    systemctl start novapanel
    log "NovaPanel service configured and started"
}

configure_nginx() {
    info "Configuring Nginx for NovaPanel..."
    cat > /etc/nginx/sites-enabled/novapanel.conf << 'NGINXEOF'
server {
    listen 2083;
    listen [::]:2083;
    server_name _;
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 86400;
    }
    location /ws {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 86400;
    }
}
NGINXEOF
    systemctl enable nginx
    systemctl restart nginx
    log "Nginx configured for NovaPanel"
}

configure_firewall() {
    info "Configuring firewall..."
    ufw --force enable
    ufw default deny incoming
    ufw default allow outgoing
    ufw allow 22/tcp
    ufw allow 80/tcp
    ufw allow 443/tcp
    ufw allow 2083/tcp
    ufw allow 25/tcp
    ufw allow 465/tcp
    ufw allow 587/tcp
    ufw allow 110/tcp
    ufw allow 143/tcp
    ufw allow 993/tcp
    ufw allow 995/tcp
    log "Firewall configured"
}

setup_nginx_templates() {
    info "Setting up Nginx templates..."
    mkdir -p /etc/novapanel/templates
    cat > /etc/novapanel/templates/default.conf << 'TEMPLATE'
server {
    listen 80;
    listen [::]:80;
    server_name __DOMAIN__ www.__DOMAIN__;
    root __DOCROOT__;
    index index.php index.html index.htm;
    access_log /var/log/nginx/__DOMAIN__.access.log;
    error_log /var/log/nginx/__DOMAIN__.error.log;
    location / { try_files $uri $uri/ /index.php?$args; }
    location ~ \.php$ {
        fastcgi_pass unix:/var/run/php/__PHP__-fpm.sock;
        fastcgi_index index.php;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }
    location ~ /\.ht { deny all; }
    location ~* \.(css|gif|ico|jpeg|jpg|js|png|svg|webp|woff|woff2|ttf|eot)$ { expires max; log_not_found off; }
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header X-Content-Type-Options "nosniff" always;
    gzip on; gzip_vary on; gzip_proxied any; gzip_comp_level 6;
    gzip_types text/plain text/css text/xml application/json application/javascript application/xml+rss application/atom+xml image/svg+xml;
}
TEMPLATE
    log "Nginx templates created"
}

configure_php() {
    info "Configuring PHP-FPM..."
    for ver in 8.1 8.2 8.3; do
        if systemctl list-units --full -all | grep -Fq "php${ver}-fpm"; then
            sed -i 's/^listen =.*/listen = \/var\/run\/php\/php'"${ver}"'-fpm.sock/' /etc/php/${ver}/fpm/pool.d/www.conf
            sed -i 's/^;listen.owner =.*/listen.owner = www-data/' /etc/php/${ver}/fpm/pool.d/www.conf
            sed -i 's/^;listen.group =.*/listen.group = www-data/' /etc/php/${ver}/fpm/pool.d/www.conf
            sed -i 's/^;listen.mode =.*/listen.mode = 0660/' /etc/php/${ver}/fpm/pool.d/www.conf
            systemctl enable "php${ver}-fpm"
            systemctl start "php${ver}-fpm"
        fi
    done
    log "PHP-FPM configured"
}

configure_postfix_dovecot() {
    info "Configuring Postfix and Dovecot..."
    postconf -e "home_mailbox = Maildir/"
    postconf -e "virtual_alias_maps = hash:/etc/postfix/virtual"
    touch /etc/postfix/virtual
    postmap /etc/postfix/virtual
    systemctl enable postfix dovecot 2>/dev/null || true
    systemctl restart postfix dovecot 2>/dev/null || true
    log "Postfix and Dovecot configured"
}

create_admin_user() {
    info "Creating admin user..."
    ${NOVAPANEL_CLI} create-admin <<EOF
admin
admin@localhost
admin123
EOF
    log "Admin user created (username: admin, password: admin123)"
}

print_summary() {
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  NovaPanel Installation Complete!${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo -e "  Panel URL:  ${CYAN}http://$(curl -s ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}'):2083${NC}"
    echo -e "  API URL:    ${CYAN}http://localhost:8080/api${NC}"
    echo -e ""
    echo -e "  Username:   ${YELLOW}admin${NC}"
    echo -e "  Password:   ${YELLOW}admin123${NC}"
    echo -e ""
    echo -e "  ${RED}IMPORTANT: Change your password after first login!${NC}"
    echo ""
    echo -e "  CLI tool:   ${CYAN}novactl${NC}"
    echo -e "  Config:     ${CYAN}/etc/novapanel/config.yaml${NC}"
    echo -e "  Logs:       ${CYAN}journalctl -u novapanel -f${NC}"
    echo ""
}

cleanup() {
    apt-get clean
    rm -f /tmp/go.tar.gz
    log "Cleanup complete"
}

main() {
    banner
    check_root
    detect_os
    setup_environment
    install_dependencies
    install_novapanel
    setup_config
    configure_nginx
    setup_nginx_templates
    configure_php
    configure_postfix_dovecot
    configure_firewall
    setup_systemd
    create_admin_user
    cleanup
    print_summary
}

main "$@"
