package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"hris/backend/internal/httputil"
	"hris/backend/internal/model"
	"hris/backend/internal/security"
)

type Service struct {
	db         *gorm.DB
	jwtSecret  string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

type AuthenticatedUser struct {
	ID          uint64          `json:"id"`
	Email       string          `json:"email"`
	DisplayName string          `json:"displayName"`
	Employee    *model.Employee `json:"employee,omitempty"`
	Roles       []string        `json:"roles"`
	Permissions []string        `json:"permissions"`
}

type TokenResult struct {
	AccessToken  string            `json:"accessToken"`
	ExpiresIn    int64             `json:"expiresIn"`
	RefreshToken string            `json:"-"`
	User         AuthenticatedUser `json:"user"`
}

func NewService(db *gorm.DB, jwtSecret string, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{db: db, jwtSecret: jwtSecret, accessTTL: accessTTL, refreshTTL: refreshTTL}
}

func (s *Service) Login(ctx context.Context, email, password string) (TokenResult, error) {
	var user model.User
	err := s.db.WithContext(ctx).Where("LOWER(email) = ?", strings.ToLower(strings.TrimSpace(email))).First(&user).Error
	if err != nil || !security.VerifyPassword(password, user.PasswordHash) {
		return TokenResult{}, httputil.NewDomainError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")
	}
	if !user.IsActive {
		return TokenResult{}, httputil.NewDomainError(http.StatusUnauthorized, "ACCOUNT_INACTIVE", "Account is inactive")
	}
	return s.issueTokens(ctx, user)
}

func (s *Service) Refresh(ctx context.Context, rawToken string) (TokenResult, error) {
	if rawToken == "" {
		return TokenResult{}, httputil.NewDomainError(http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Refresh token is required")
	}
	var result TokenResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session model.RefreshSession
		if err := tx.Where("token_hash = ? AND revoked_at IS NULL", security.HashToken(rawToken)).First(&session).Error; err != nil {
			return httputil.NewDomainError(http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Refresh token is invalid or revoked")
		}
		if time.Now().UTC().After(session.ExpiresAt) {
			return httputil.NewDomainError(http.StatusUnauthorized, "REFRESH_TOKEN_EXPIRED", "Refresh token has expired")
		}
		var user model.User
		if err := tx.First(&user, session.UserID).Error; err != nil || !user.IsActive {
			return httputil.NewDomainError(http.StatusUnauthorized, "ACCOUNT_UNAVAILABLE", "Account is unavailable")
		}
		now := time.Now().UTC()
		if err := tx.Model(&session).Update("revoked_at", now).Error; err != nil {
			return err
		}
		issued, err := s.issueTokensWithDB(ctx, tx, user)
		if err != nil {
			return err
		}
		result = issued
		return nil
	})
	return result, err
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	return s.db.WithContext(ctx).Model(&model.RefreshSession{}).
		Where("token_hash = ? AND revoked_at IS NULL", security.HashToken(rawToken)).
		Update("revoked_at", time.Now().UTC()).Error
}

func (s *Service) Me(ctx context.Context, userID uint64) (AuthenticatedUser, error) {
	var user model.User
	if err := s.db.WithContext(ctx).Preload("Employee.Department").Preload("Employee.Position").First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AuthenticatedUser{}, httputil.NewDomainError(http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		}
		return AuthenticatedUser{}, err
	}
	return s.userPayload(ctx, s.db, user)
}

func (s *Service) issueTokens(ctx context.Context, user model.User) (TokenResult, error) {
	var result TokenResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		issued, err := s.issueTokensWithDB(ctx, tx, user)
		result = issued
		return err
	})
	return result, err
}

func (s *Service) issueTokensWithDB(ctx context.Context, tx *gorm.DB, user model.User) (TokenResult, error) {
	access, err := security.CreateAccessToken(user.ID, s.jwtSecret, s.accessTTL)
	if err != nil {
		return TokenResult{}, err
	}
	raw, hash, err := security.NewOpaqueToken()
	if err != nil {
		return TokenResult{}, err
	}
	session := model.RefreshSession{UserID: user.ID, TokenHash: hash, ExpiresAt: time.Now().UTC().Add(s.refreshTTL)}
	if err := tx.WithContext(ctx).Create(&session).Error; err != nil {
		return TokenResult{}, err
	}
	payload, err := s.userPayload(ctx, tx, user)
	if err != nil {
		return TokenResult{}, err
	}
	return TokenResult{AccessToken: access, ExpiresIn: int64(s.accessTTL.Seconds()), RefreshToken: raw, User: payload}, nil
}

func (s *Service) userPayload(ctx context.Context, db *gorm.DB, user model.User) (AuthenticatedUser, error) {
	if user.Employee == nil {
		var employee model.Employee
		if err := db.WithContext(ctx).Preload("Department").Preload("Position").Where("user_id = ?", user.ID).First(&employee).Error; err == nil {
			user.Employee = &employee
		}
	}
	var roles, permissions []string
	if err := db.WithContext(ctx).Table("roles r").Joins("JOIN user_roles ur ON ur.role_id = r.id").Where("ur.user_id = ?", user.ID).Order("r.name").Pluck("r.name", &roles).Error; err != nil {
		return AuthenticatedUser{}, err
	}
	if err := db.WithContext(ctx).Table("permissions p").Distinct("p.code").Joins("JOIN role_permissions rp ON rp.permission_id = p.id").Joins("JOIN user_roles ur ON ur.role_id = rp.role_id").Where("ur.user_id = ?", user.ID).Order("p.code").Pluck("p.code", &permissions).Error; err != nil {
		return AuthenticatedUser{}, err
	}
	displayName := user.Email
	if user.Employee != nil {
		displayName = user.Employee.FullName
	}
	return AuthenticatedUser{ID: user.ID, Email: user.Email, DisplayName: displayName, Employee: user.Employee, Roles: roles, Permissions: permissions}, nil
}
