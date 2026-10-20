package command

import (
	"context"
	"time"

	infratenant "github.com/stanley760/devops/devops-infra/tenant"

	"github.com/stanley760/devops/devops-system/internal/domain/iam"
	"github.com/stanley760/devops/devops-system/internal/domain/shared"
)

// TenantCommand is the write-side tenant command handler.
type TenantCommand struct {
	tenants iam.TenantRepository
	users   iam.UserRepository
	roles   iam.RoleRepository
	txs     shared.TxManager
	now     shared.NowFunc
	// SuperAdminRole is the built-in super-admin role code (injected from the authz role-name config).
	SuperAdminRole func() string
	// HashPassword hashes passwords (bcrypt in production).
	HashPassword func(string) (string, error)
}

// NewTenantCommand constructs the tenant command handler.
func NewTenantCommand(
	tenants iam.TenantRepository,
	users iam.UserRepository,
	roles iam.RoleRepository,
	txs shared.TxManager,
) *TenantCommand {
	return &TenantCommand{
		tenants: tenants, users: users, roles: roles,
		txs: txs, now: shared.SystemNow,
	}
}

// CreateTenantCmd is the create-tenant command (includes the initial super admin).
type CreateTenantCmd struct {
	Name          string
	Code          string
	ContactName   string
	ContactMobile string
	ExpireTime    *time.Time
	Remark        string
	AdminUsername string
	AdminPassword string
}

// Create creates a tenant + initial super admin (role + admin account).
//
// This is where the refactor pays off most: before it
// (service/tenant.go:104-108), the transaction boundary was locked inside
// the dao package, so the application layer could not declare "create
// tenant + create super admin" as one transaction and had to rely on manual
// compensation —
//
//	if err := dao.SeedTenantAdmin(tctx, role, admin); err != nil {
//	    _ = dao.HardDeleteTenant(ctx, t.ID) // compensation: roll back the tenant row
//	    return nil, err
//	}
//
// That compensation was itself flawed: a failed compensation was swallowed
// by `_ =`, leaving an unusable "tenant without super admin" state; and
// because the compensation was not in the same transaction as the original
// write, it could delete other people's data under concurrency.
//
// Now the transaction boundary is declared by TxManager, and the tenant row,
// role row, admin row, role binding, and domain events commit in a single
// transaction — all or nothing.
//
// Note: the initial super-admin authorization (casbin p/g rows) still goes
// through event projection, outside this transaction, so the vague error
// "tenant created, but super-admin authorization failed, please retry"
// no longer occurs.
func (c *TenantCommand) Create(ctx context.Context, cmd CreateTenantCmd) error {
	exists, err := c.tenants.CodeExists(ctx, cmd.Code, 0)
	if err != nil {
		return err
	}
	if exists {
		return errParam("tenant code already exists")
	}

	hash, err := hashPassword(cmd.AdminPassword, c.HashPassword)
	if err != nil {
		return err
	}

	now := c.now()
	t := &iam.Tenant{
		Name:          cmd.Name,
		Code:          cmd.Code,
		ContactName:   cmd.ContactName,
		ContactMobile: cmd.ContactMobile,
		ExpireTime:    cmd.ExpireTime,
		Remark:        cmd.Remark,
		Status:        iam.TenantStatusEnabled,
	}

	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		if err := c.tenants.Save(ctx, t); err != nil {
			return err
		}

		// Writes to tenant-scoped tables still need the new tenant context injected; TenantPlugin uses it to fill tenant_id.
		tctx := infratenant.With(ctx, t.ID, t.Code)

		role := &iam.Role{
			TenantID:  t.ID,
			Name:      "超级管理员",
			Code:      c.SuperAdminRole(),
			Type:      iam.RoleTypeBuiltin,
			DataScope: iam.DataScopeAll,
			Status:    1,
		}
		if err := c.roles.Save(tctx, role); err != nil {
			return err
		}

		admin := &iam.User{
			TenantID: t.ID,
			Username: cmd.AdminUsername,
			Password: hash,
			Nickname: cmd.AdminUsername,
			Status:   iam.UserStatusEnabled,
		}
		// Assign the super-admin role: record the UserRolesAssigned event, from which the projector builds the casbin g rows.
		admin.AssignRoles([]int64{role.ID}, []string{role.Code}, now)
		if err := c.users.Save(tctx, admin); err != nil {
			return err
		}

		// Initial super-admin authorization (domain-wide p rows + g rows): record an
		// event and let the projector execute it. Before the refactor this called
		// authz.SeedTenantAdmin synchronously here, returning "tenant created, but
		// super-admin authorization failed, please retry" on failure.
		role.RecordEvent(iam.TenantAdminSeeded{
			BaseEvent:   shared.BaseEvent{Time: now},
			TenantID:    t.ID,
			AdminUserID: admin.ID,
		})
		return c.roles.Save(tctx, role)
	})
}

// UpdateTenantCmd is the update-tenant command.
type UpdateTenantCmd struct {
	TenantID      int64
	Name          string
	ContactName   string
	ContactMobile string
	ExpireTime    *time.Time
	Remark        string
}

// Update updates a tenant.
//
// Business rule: if an expiry time is provided, the status is derived from
// it (future → enabled, past → disabled); if omitted, the expiry time is
// cleared (never expires). This rule used to live in service/tenant.go:130-140
// as an updates map and is now a domain method.
func (c *TenantCommand) Update(ctx context.Context, cmd UpdateTenantCmd) error {
	t, err := c.tenants.Get(ctx, cmd.TenantID)
	if err != nil {
		return err
	}

	now := c.now()
	t.Change(iam.TenantUpdate{
		Name:          cmd.Name,
		ContactName:   cmd.ContactName,
		ContactMobile: cmd.ContactMobile,
		ExpireTime:    cmd.ExpireTime,
		Remark:        cmd.Remark,
	}, now)
	t.SyncStatusWithExpiry(now)

	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		return c.tenants.Save(ctx, t)
	})
}

// ChangeTenantStatusCmd is the change-tenant-status command.
type ChangeTenantStatusCmd struct {
	TenantID int64
	Status   int8
}

// ChangeStatus enables or disables a tenant.
//
// On disable the aggregate records a TenantDisabled event, and the projector
// revokes all of that tenant's sessions (everyone goes offline); the main
// flow no longer waits synchronously for session revocation to finish.
func (c *TenantCommand) ChangeStatus(ctx context.Context, cmd ChangeTenantStatusCmd) error {
	t, err := c.tenants.Get(ctx, cmd.TenantID)
	if err != nil {
		return err
	}
	t.Change(iam.TenantUpdate{
		Name:          t.Name,
		ContactName:   t.ContactName,
		ContactMobile: t.ContactMobile,
		ExpireTime:    t.ExpireTime,
		Remark:        t.Remark,
		Status:        cmd.Status,
	}, c.now())

	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		return c.tenants.Save(ctx, t)
	})
}

// DeleteTenantCmd is the delete-tenant command.
type DeleteTenantCmd struct {
	TenantID int64
}

// Delete soft-deletes a tenant.
func (c *TenantCommand) Delete(ctx context.Context, cmd DeleteTenantCmd) error {
	t, err := c.tenants.Get(ctx, cmd.TenantID)
	if err != nil {
		return err
	}
	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		return c.tenants.Delete(ctx, t, t.Code)
	})
}
