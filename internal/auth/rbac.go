package auth

type Role string

const (
	RoleAdmin   Role = "admin"
	RoleReseller Role = "reseller"
	RoleClient  Role = "client"
)

type Permission string

const (
	PermManageUsers      Permission = "manage_users"
	PermManageDomains    Permission = "manage_domains"
	PermManageDatabases  Permission = "manage_databases"
	PermManageEmail      Permission = "manage_email"
	PermManageFirewall   Permission = "manage_firewall"
	PermManageBackups    Permission = "manage_backups"
	PermManageSystem     Permission = "manage_system"
	PermManageCron       Permission = "manage_cron"
	PermViewMonitoring   Permission = "view_monitoring"
	PermManageFiles      Permission = "manage_files"
	PermManageSSL        Permission = "manage_ssl"
)

var rolePermissions = map[Role][]Permission{
	RoleAdmin: {
		PermManageUsers, PermManageDomains, PermManageDatabases,
		PermManageEmail, PermManageFirewall, PermManageBackups,
		PermManageSystem, PermManageCron, PermViewMonitoring,
		PermManageFiles, PermManageSSL,
	},
	RoleReseller: {
		PermManageUsers, PermManageDomains, PermManageDatabases,
		PermManageEmail, PermManageBackups, PermManageCron,
		PermViewMonitoring, PermManageFiles, PermManageSSL,
	},
	RoleClient: {
		PermManageDomains, PermManageDatabases, PermManageEmail,
		PermManageCron, PermViewMonitoring, PermManageFiles, PermManageSSL,
	},
}

func HasPermission(role Role, permission Permission) bool {
	for _, p := range rolePermissions[role] {
		if p == permission {
			return true
		}
	}
	return false
}

func GetRolePermissions(role Role) []Permission {
	return rolePermissions[role]
}
