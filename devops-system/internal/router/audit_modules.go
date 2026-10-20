package router

import (
	"github.com/stanley760/devops/devops-infra/security/humakit"
)

// Audit module names (written to the module / sub_module columns of t_system_operation_log).
const (
	auditModuleSystem = "system"
)

// auditedOperations declares the routes included in the operation audit (OperationID → sub-module).
//
// Inclusion principles:
//   - only write operations (POST/PUT/DELETE) and sensitive read operations are audited;
//     GET list/detail are not audited, to avoid high-frequency polling bloating the audit table.
//   - the login audit is carried independently by t_system_login_log (recording both success and failure attempts,
//     including scenarios failing authentication), so auth-login is not registered again here.
//   - the audit table's own query and clean are not audited (self-auditing is pointless and amplifies writes).
var auditedOperations = map[string]string{
	// users
	"user-create":         "user",
	"user-update":         "user",
	"user-delete":         "user",
	"user-reset-password": "user",
	// roles
	"role-create":       "role",
	"role-update":       "role",
	"role-delete":       "role",
	"role-assign-menus": "role",
	// menus
	"menu-create": "menu",
	"menu-update": "menu",
	"menu-delete": "menu",
	// depts
	"dept-create": "dept",
	"dept-update": "dept",
	"dept-delete": "dept",
	// posts
	"post-create": "post",
	"post-update": "post",
	"post-delete": "post",
	// tenants (platform level)
	"tenant-create":        "tenant",
	"tenant-update":        "tenant",
	"tenant-delete":        "tenant",
	"tenant-update-status": "tenant",
	// personal account
	"auth-update-profile":  "profile",
	"auth-update-password": "profile",
}

// auditModuleResolver resolves an OperationID to an audit module.
//
// The assembly point (cmd layer) passes this function via humakit.EnableAudit: the audit
// middleware decides which routes to persist by this whitelist. When adding a new write
// operation, just register one line here, without changing handlers.
func auditModuleResolver(operationID string) (module, subModule string, ok bool) {
	subModule, ok = auditedOperations[operationID]
	if !ok {
		return "", "", false
	}
	return auditModuleSystem, subModule, true
}

// AuditModuleResolver is exported for the cmd assembly layer:
// the router package holds the table of "which routes should be audited"; cmd only injects the
// Sink, keeping the two responsibilities unmixed.
func AuditModuleResolver() humakit.ModuleResolver { return auditModuleResolver }

// compile-time assertion: this module's resolver satisfies the infra-side contract.
var _ humakit.ModuleResolver = auditModuleResolver
