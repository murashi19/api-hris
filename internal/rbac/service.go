package rbac

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"hris/backend/internal/audit"
	"hris/backend/internal/httputil"
	"hris/backend/internal/model"
	"hris/backend/internal/security"
)

type Service struct{ db *gorm.DB }

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

type CreateUserInput struct {
	Email    string   `json:"email" binding:"required,email,max=255"`
	Password string   `json:"password" binding:"required,min=8,max=200"`
	IsActive *bool    `json:"isActive"`
	RoleIDs  []uint64 `json:"roleIds"`
}
type RoleInput struct {
	Name          string   `json:"name" binding:"required,max=80"`
	Description   string   `json:"description" binding:"max=255"`
	PermissionIDs []uint64 `json:"permissionIds"`
}

func (s *Service) Users(ctx context.Context, page, limit, offset int) ([]model.User, int64, error) {
	q := s.db.WithContext(ctx).Model(&model.User{})
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.User
	err := q.Preload("Employee").Preload("Roles").Order("email").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}
func (s *Service) CreateUser(ctx context.Context, in CreateUserInput, actor uint64) (model.User, error) {
	hash, err := security.HashPassword(in.Password)
	if err != nil {
		return model.User{}, httputil.NewDomainError(422, "INVALID_PASSWORD", err.Error())
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	user := model.User{Email: strings.ToLower(strings.TrimSpace(in.Email)), PasswordHash: hash, IsActive: active}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(in.RoleIDs) > 0 {
			var roles []model.Role
			if err := tx.Where("id IN ?", in.RoleIDs).Find(&roles).Error; err != nil {
				return err
			}
			if len(roles) != len(in.RoleIDs) {
				return httputil.NewDomainError(422, "INVALID_ROLE", "One or more roles do not exist")
			}
			user.Roles = roles
		}
		if err := tx.Create(&user).Error; err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
				return httputil.NewDomainError(409, "USER_ALREADY_EXISTS", "User email already exists")
			}
			return err
		}
		return audit.Write(ctx, tx, &actor, "USER_CREATED", "user", user.ID, map[string]any{"email": user.Email})
	})
	return user, err
}
func (s *Service) AssignRoles(ctx context.Context, userID uint64, roleIDs []uint64, actor uint64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.First(&user, userID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return httputil.NewDomainError(http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		} else if err != nil {
			return err
		}
		var roles []model.Role
		if len(roleIDs) > 0 {
			if err := tx.Where("id IN ?", roleIDs).Find(&roles).Error; err != nil {
				return err
			}
			if len(roles) != len(roleIDs) {
				return httputil.NewDomainError(422, "INVALID_ROLE", "One or more roles do not exist")
			}
		}
		if err := tx.Model(&user).Association("Roles").Replace(roles); err != nil {
			return err
		}
		return audit.Write(ctx, tx, &actor, "ROLE_ASSIGNED", "user", userID, map[string]any{"roleIds": roleIDs})
	})
}
func (s *Service) Roles(ctx context.Context) ([]model.Role, error) {
	var roles []model.Role
	err := s.db.WithContext(ctx).Preload("Permissions").Order("name").Find(&roles).Error
	return roles, err
}
func (s *Service) Permissions(ctx context.Context) ([]model.Permission, error) {
	var items []model.Permission
	err := s.db.WithContext(ctx).Order("code").Find(&items).Error
	return items, err
}
func (s *Service) CreateRole(ctx context.Context, in RoleInput, actor uint64) (model.Role, error) {
	role := model.Role{Name: strings.TrimSpace(in.Name), Description: strings.TrimSpace(in.Description)}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var permissions []model.Permission
		if len(in.PermissionIDs) > 0 {
			if err := tx.Where("id IN ?", in.PermissionIDs).Find(&permissions).Error; err != nil {
				return err
			}
			if len(permissions) != len(in.PermissionIDs) {
				return httputil.NewDomainError(422, "INVALID_PERMISSION", "One or more permissions do not exist")
			}
			role.Permissions = permissions
		}
		if err := tx.Create(&role).Error; err != nil {
			return err
		}
		return audit.Write(ctx, tx, &actor, "ROLE_CREATED", "role", role.ID, map[string]any{"name": role.Name})
	})
	return role, err
}
