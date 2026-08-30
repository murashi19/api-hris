package department

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"hris/backend/internal/audit"
	"hris/backend/internal/httputil"
	"hris/backend/internal/model"
)

type Service struct{ db *gorm.DB }

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

type Input struct {
	Name     string `json:"name" binding:"required,max=120"`
	IsActive *bool  `json:"isActive"`
}

func (s *Service) List(ctx context.Context) ([]model.Department, error) {
	var items []model.Department
	err := s.db.WithContext(ctx).Order("name").Find(&items).Error
	return items, err
}
func (s *Service) Create(ctx context.Context, input Input, actor uint64) (model.Department, error) {
	active := true
	if input.IsActive != nil {
		active = *input.IsActive
	}
	item := model.Department{Name: strings.TrimSpace(input.Name), IsActive: active}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
				return httputil.NewDomainError(409, "DEPARTMENT_ALREADY_EXISTS", "Department name already exists")
			}
			return err
		}
		return audit.Write(ctx, tx, &actor, "DEPARTMENT_CREATED", "department", item.ID, map[string]any{"name": item.Name})
	})
	return item, err
}
func (s *Service) Update(ctx context.Context, id uint64, input Input, actor uint64) (model.Department, error) {
	var item model.Department
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&item, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return httputil.NewDomainError(http.StatusNotFound, "DEPARTMENT_NOT_FOUND", "Department not found")
		} else if err != nil {
			return err
		}
		updates := map[string]any{"name": strings.TrimSpace(input.Name)}
		if input.IsActive != nil {
			updates["is_active"] = *input.IsActive
		}
		if err := tx.Model(&item).Updates(updates).Error; err != nil {
			return err
		}
		return audit.Write(ctx, tx, &actor, "DEPARTMENT_UPDATED", "department", id, updates)
	})
	return item, err
}
