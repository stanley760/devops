package dao

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-user/internal/model"
)

// GetOAuthIdentity looks up a third-party binding by
// (provider, provider_user_id). Returning nil, nil means not yet bound (not an
// error).
func GetOAuthIdentity(ctx context.Context, provider model.OAuthProvider, providerUserID string) (*model.OAuthIdentity, error) {
	var m model.OAuthIdentity
	err := db(ctx).Where("provider = ? AND provider_user_id = ?", provider, providerUserID).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query third-party binding")
	}
	return &m, nil
}

// ListOAuthIdentities lists all third-party bindings of a user (for the
// settings page).
func ListOAuthIdentities(ctx context.Context, userID int64) ([]model.OAuthIdentity, error) {
	var list []model.OAuthIdentity
	if err := db(ctx).Where("user_id = ?", userID).Find(&list).Error; err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query third-party binding list")
	}
	return list, nil
}

// CreateOAuthIdentity creates a binding.
//
// Unique conflicts are converted into expected business errors: a
// uk_provider_provider_id conflict means the third-party account is already
// bound to another local user (an account hijacking signal, **must reject**).
func CreateOAuthIdentity(ctx context.Context, m *model.OAuthIdentity) error {
	if err := db(ctx).Create(m).Error; err != nil {
		if dup, index := duplicateField(err); dup {
			switch index {
			case "uk_provider_provider_id":
				return errs.New(errs.CodeParam, "this third-party account is already bound to another user")
			case "uk_user_provider":
				return errs.New(errs.CodeParam, "this third-party provider is already bound")
			default:
				return errs.New(errs.CodeParam, "binding information conflict")
			}
		}
		return errs.Wrap(err, errs.CodeInternal, "failed to create third-party binding")
	}
	return nil
}

// UpdateOAuthToken refreshes the third-party token and display info (the
// snapshot is updated on every login).
func UpdateOAuthToken(ctx context.Context, id int64, accessToken, refreshToken string, expiresAt int64, nickname, avatarURL string) error {
	updates := map[string]any{
		"access_token": accessToken,
		"expires_at":   expiresAt,
	}
	if refreshToken != "" {
		updates["refresh_token"] = refreshToken
	}
	if nickname != "" {
		updates["nickname"] = nickname
	}
	if avatarURL != "" {
		updates["avatar_url"] = avatarURL
	}
	if err := db(ctx).Model(&model.OAuthIdentity{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to update third-party token")
	}
	return nil
}

// DeleteOAuthIdentity unbinds (initiated by the user or by the admin side).
func DeleteOAuthIdentity(ctx context.Context, userID int64, provider model.OAuthProvider) error {
	res := db(ctx).Where("user_id = ? AND provider = ?", userID, provider).Delete(&model.OAuthIdentity{})
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to unbind")
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeNotFound, "this third-party account is not bound")
	}
	return nil
}
