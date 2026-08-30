package position

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"hris/backend/internal/audit"
	"hris/backend/internal/httputil"
	"hris/backend/internal/model"
	"net/http"
	"strings"
)

type Service struct{ db *gorm.DB }

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

type Input struct {
	Name         string  `json:"name" binding:"required,max=120"`
	DepartmentID *uint64 `json:"departmentId"`
	IsActive     *bool   `json:"isActive"`
}

func (s *Service) List(ctx context.Context) ([]model.Position, error) {
	var items []model.Position
	err := s.db.WithContext(ctx).Preload("Department").Order("name").Find(&items).Error
	return items, err
}
func (s *Service) validateDepartment(ctx context.Context, id *uint64) error {
	if id == nil {
		return nil
	}
	var n int64
	s.db.WithContext(ctx).Model(&model.Department{}).Where("id=? AND is_active=true", *id).Count(&n)
	if n == 0 {
		return httputil.NewDomainError(422, "INVALID_DEPARTMENT", "Active department does not exist")
	}
	return nil
}
func (s *Service) Create(ctx context.Context, in Input, actor uint64) (model.Position, error) {
	if err := s.validateDepartment(ctx, in.DepartmentID); err != nil {
		return model.Position{}, err
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	item := model.Position{Name: strings.TrimSpace(in.Name), DepartmentID: in.DepartmentID, IsActive: active}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
				return httputil.NewDomainError(409, "POSITION_ALREADY_EXISTS", "Position already exists in this department")
			}
			return err
		}
		return audit.Write(ctx, tx, &actor, "POSITION_CREATED", "position", item.ID, map[string]any{"name": item.Name})
	})
	return item, err
}
func (s *Service) Update(ctx context.Context, id uint64, in Input, actor uint64) (model.Position, error) {
	if err := s.validateDepartment(ctx, in.DepartmentID); err != nil {
		return model.Position{}, err
	}
	var item model.Position
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&item, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return httputil.NewDomainError(http.StatusNotFound, "POSITION_NOT_FOUND", "Position not found")
		} else if err != nil {
			return err
		}
		updates := map[string]any{"name": strings.TrimSpace(in.Name), "department_id": in.DepartmentID}
		if in.IsActive != nil {
			updates["is_active"] = *in.IsActive
		}
		if err := tx.Model(&item).Updates(updates).Error; err != nil {
			return err
		}
		return audit.Write(ctx, tx, &actor, "POSITION_UPDATED", "position", id, updates)
	})
	return item, err
}
