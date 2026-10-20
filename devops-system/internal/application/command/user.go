// Package command is the CQRS write side: command definitions and handlers.
//
// The write side uniformly follows "load aggregate → invoke aggregate
// behavior → repository save (including event persistence)" and never
// touches the query model directly. The transaction boundary is declared
// explicitly by shared.TxManager.
//
// Three key improvements over the pre-refactor service layer:
//  1. The transaction boundary moved up to the application layer, so
//     cross-aggregate use cases can be atomic (previously only manual
//     compensation was possible);
//  2. Business rules live inside the aggregates (rich domain model); the
//     handlers only orchestrate and no longer contain business ifs;
//  3. casbin authorization sync is now driven by domain events, so failures
//     do not affect the main-flow response.
package command

import (
	"context"

	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/domain/iam"
	"github.com/stanley760/devops/devops-system/internal/domain/shared"
)

// errParam builds a parameter error (400).
func errParam(msg string) error { return errs.New(errs.CodeParam, msg) }

// hashPassword computes the password hash.
//
// Injection point: production uses bcrypt, while tests can pass a double,
// letting command handlers be unit-tested end to end without a database.
func hashPassword(plain string, hasher func(string) (string, error)) (string, error) {
	if hasher == nil {
		return "", errs.New(errs.CodeInternal, "password hash function not wired up")
	}
	h, err := hasher(plain)
	if err != nil {
		return "", errs.Wrap(err, errs.CodeInternal, "password encryption failed")
	}
	return h, nil
}

// UserCommand is the write-side user command handler.
//
// All dependencies are ports (interfaces), so in-memory fakes can be injected
// for unit tests — this is exactly why the pre-refactor service layer had
// zero test coverage: the dao exposed package-level functions, making
// doubles impossible to inject.
type UserCommand struct {
	users iam.UserRepository
	txs   shared.TxManager
	now   shared.NowFunc
}

// NewUserCommand constructs the user command handler.
func NewUserCommand(users iam.UserRepository, txs shared.TxManager) *UserCommand {
	return &UserCommand{users: users, txs: txs, now: shared.SystemNow}
}

// CreateUserCmd is the create-user command.
type CreateUserCmd struct {
	// TenantID is the target tenant (parsed from claims, passed in by the handler layer).
	TenantID int64
	Username string
	// Password is the plaintext password; the handler encrypts it, and the domain layer only ever sees the digest.
	Password string
	Nickname string
	Remark   string
	DeptID   int64
	PostIDs  []int64
	Email    string
	Mobile   *string
	Gender   int8
	Avatar   string
	Status   int8
	RoleIDs  []int64
	// HashPassword is the password hash function (injected for testability; defaults to bcrypt).
	HashPassword func(string) (string, error)
}

// CreateUser creates a user.
//
// Consistency: the user row, role associations, and domain events commit in a
// single transaction. Authorization (casbin g rows) is done by the projector
// for the UserRolesAssigned event, outside this transaction, so the
// semantically vague error "user created but authorization failed, please
// retry" no longer occurs.
func (c *UserCommand) Create(ctx context.Context, cmd CreateUserCmd) error {
	hash, err := hashPassword(cmd.Password, cmd.HashPassword)
	if err != nil {
		return err
	}

	// Uniqueness pre-check: a fast-path rejection; real uniqueness is
	// guaranteed by the database unique index (under concurrency two requests
	// can pass the pre-check simultaneously; uk_tenant_username keeps them
	// from duplicating).
	exists, err := c.users.UsernameExists(ctx, cmd.Username, 0)
	if err != nil {
		return err
	}
	if exists {
		return errParam("username already exists")
	}
	if cmd.Mobile != nil && *cmd.Mobile != "" {
		exists, err = c.users.MobileExists(ctx, *cmd.Mobile, 0)
		if err != nil {
			return err
		}
		if exists {
			return errParam("mobile number already exists")
		}
	}

	codes, err := c.users.RoleCodesByIDs(ctx, cmd.RoleIDs)
	if err != nil {
		return err
	}

	now := c.now()
	u := &iam.User{
		TenantID: cmd.TenantID,
		Username: cmd.Username,
		Password: hash,
		Nickname: cmd.Nickname,
		Remark:   cmd.Remark,
		DeptID:   cmd.DeptID,
		PostIDs:  cmd.PostIDs,
		Email:    cmd.Email,
		Mobile:   cmd.Mobile,
		Gender:   cmd.Gender,
		Avatar:   cmd.Avatar,
		Status:   iam.UserStatus(cmd.Status),
	}
	if err := u.ValidateForCreate(); err != nil {
		return err
	}
	u.AssignRoles(cmd.RoleIDs, codes, now)

	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		return c.users.Save(ctx, u)
	})
}

// UpdateUserCmd is the update-user command.
type UpdateUserCmd struct {
	TenantID     int64
	UserID       int64
	Username     string
	Nickname     string
	Remark       string
	DeptID       int64
	PostIDs      []int64
	Email        string
	Mobile       *string
	Gender       int8
	Avatar       string
	Status       int8
	RoleIDs      []int64
	ActorID      int64
	HashPassword func(string) (string, error)
}

// UpdateUser updates a user (profile + roles + status).
//
// All rules are owned by the aggregate: a role change records an event
// (triggering g-row sync and data-scope cache invalidation), and a 1→0
// status change records a UserDeactivated event (triggering a forced
// logout).
func (c *UserCommand) Update(ctx context.Context, cmd UpdateUserCmd) error {
	u, err := c.users.Get(ctx, cmd.UserID)
	if err != nil {
		return err
	}

	// Uniqueness: only validate when the value has changed (avoids pointless queries).
	if u.Username != cmd.Username {
		exists, err := c.users.UsernameExists(ctx, cmd.Username, u.ID)
		if err != nil {
			return err
		}
		if exists {
			return errParam("username already exists")
		}
	}
	if cmd.Mobile != nil && *cmd.Mobile != "" && (u.Mobile == nil || *u.Mobile != *cmd.Mobile) {
		exists, err := c.users.MobileExists(ctx, *cmd.Mobile, u.ID)
		if err != nil {
			return err
		}
		if exists {
			return errParam("mobile number already exists")
		}
	}

	codes, err := c.users.RoleCodesByIDs(ctx, cmd.RoleIDs)
	if err != nil {
		return err
	}

	now := c.now()
	u.Username = cmd.Username
	u.ChangeProfile(iam.ProfileUpdate{
		Nickname: cmd.Nickname,
		Email:    cmd.Email,
		Remark:   cmd.Remark,
		DeptID:   cmd.DeptID,
		Gender:   cmd.Gender,
		Avatar:   cmd.Avatar,
		Mobile:   cmd.Mobile,
		PostIDs:  cmd.PostIDs,
	})
	u.AssignRoles(cmd.RoleIDs, codes, now)
	u.ChangeStatus(iam.UserStatus(cmd.Status), now)

	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		return c.users.Save(ctx, u)
	})
}

// ResetPasswordCmd is the reset-password command.
type ResetPasswordCmd struct {
	TenantID     int64
	UserID       int64
	Password     string
	HashPassword func(string) (string, error)
}

// ResetPassword resets a password.
//
// A reset forces re-login — the UserPasswordChanged event triggers
// revocation of all sessions, and the domain layer never calls auth
// directly (keeping the domain layer free of infrastructure dependencies).
func (c *UserCommand) ResetPassword(ctx context.Context, cmd ResetPasswordCmd) error {
	u, err := c.users.Get(ctx, cmd.UserID)
	if err != nil {
		return err
	}
	if err := u.EnsurePasswordResettable(); err != nil {
		return err
	}
	hash, err := hashPassword(cmd.Password, cmd.HashPassword)
	if err != nil {
		return err
	}
	u.ChangePassword(hash, c.now())

	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		return c.users.Save(ctx, u)
	})
}

// DeleteUserCmd is the delete-user command.
type DeleteUserCmd struct {
	TenantID int64
	UserID   int64
	// ActorID is the operator; used by the "cannot delete yourself" domain check.
	ActorID int64
}

// DeleteUser soft-deletes a user.
func (c *UserCommand) Delete(ctx context.Context, cmd DeleteUserCmd) error {
	u, err := c.users.Get(ctx, cmd.UserID)
	if err != nil {
		return err
	}
	if err := u.EnsureDeletable(cmd.ActorID); err != nil {
		return err
	}
	now := c.now()
	u.MarkDeleted(now)

	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		return c.users.Delete(ctx, u, u.Username)
	})
}

// UpdateProfileCmd is the update-own-profile command.
type UpdateProfileCmd struct {
	TenantID int64
	UserID   int64
	Nickname string
	Email    string
	Mobile   *string
	Gender   int8
	Avatar   string
}

// UpdateProfile updates the current user's profile (roles and status unchanged).
func (c *UserCommand) UpdateProfile(ctx context.Context, cmd UpdateProfileCmd) error {
	u, err := c.users.Get(ctx, cmd.UserID)
	if err != nil {
		return err
	}
	u.ChangeProfile(iam.ProfileUpdate{
		Nickname: cmd.Nickname,
		Email:    cmd.Email,
		Gender:   cmd.Gender,
		Avatar:   cmd.Avatar,
		Mobile:   cmd.Mobile,
	})
	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		return c.users.Save(ctx, u)
	})
}
