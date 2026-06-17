# NovaPanel - Server Control Panel

A production-grade web hosting control panel for Ubuntu Linux, similar to cPanel/DirectAdmin. Built with Go, React, and modern system integration.

## Features

- **Web Server Management** - Nginx virtual hosts, SSL, PHP version selector
- **File Manager** - Web-based file explorer with edit/upload/zip
- **Database Management** - MySQL/MariaDB via web UI
- **Email Management** - Postfix + Dovecot integration
- **Firewall** - UFW frontend with rule management
- **Backups** - Full/site/database backups
- **Server Monitoring** - CPU, RAM, disk, processes, services
- **Cron Jobs** - UI-based cron management
- **Security** - JWT auth, RBAC, audit logging, rate limiting
- **CLI Tool** - `novactl` for server administration

## Quick Install

```bash
curl -sSL https://get.novapanel.io/install.sh | bash
```

Or build from source:

```bash
git clone https://github.com/novapanel/novapanel.git
cd novapanel
go build -o novapanel ./cmd/novapanel/
go build -o novactl ./cmd/novactl/
```

## Architecture

```
├── cmd/
│   ├── novapanel/        # Server entry point
│   └── novactl/          # CLI tool
├── internal/
│   ├── api/              # REST API handlers & router
│   ├── auth/             # JWT, passwords, RBAC
│   ├── db/               # GORM models & migrations
│   ├── hosting/          # Nginx, SSL, PHP management
│   ├── system/           # Systemd, executor, audit
│   ├── security/         # Firewall, fail2ban
│   ├── monitor/          # Server monitoring
│   ├── email/            # Postfix/Dovecot management
│   ├── backup/           # Backup system
│   └── config/           # Configuration
├── frontend/             # React dashboard
├── scripts/              # Installer
└── config/               # Config files
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | /api/login | User login |
| GET | /api/me | Current user info |
| GET | /api/users | List users |
| POST | /api/users | Create user |
| GET/POST/DELETE | /api/domains | Domain management |
| GET/POST | /api/databases | Database management |
| GET/POST/DELETE | /api/email | Email accounts |
| GET/POST/DELETE | /api/firewall | Firewall rules |
| GET/POST | /api/backups | Backup management |
| GET | /api/system/stats | Server statistics |
| GET | /api/system/services | Service status |
| GET/POST/DELETE | /api/cron | Cron jobs |
| GET | /api/files | File manager |

## CLI Usage

```bash
novactl install          # Install and initialize
novactl create-admin     # Create admin user
novactl server status    # Show server status
novactl version          # Show version
```

## Security

- JWT authentication with access/refresh tokens
- Role-based access control (admin, reseller, client)
- Rate limiting on all endpoints
- CSRF protection
- Input sanitization
- Audit logging for all actions
- Security headers (CSP, HSTS, X-Frame-Options)
- Path traversal prevention in file manager

## Requirements

- Ubuntu 20.04 LTS or newer
- Systemd
- Nginx (installed automatically)
- MySQL/MariaDB (installed automatically)

## Configuration

Config file: `/etc/novapanel/config.yaml`

Key settings:
- `server.port` - API port (default: 8080)
- `server.panel_port` - Panel web port (default: 2083)
- `jwt.secret` - JWT signing secret
- `database.driver` - sqlite or postgres

## License

MIT
