package employee

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"hris/backend/internal/audit"
	"hris/backend/internal/httputil"
	"hris/backend/internal/model"
)

type Service struct{ db *gorm.DB }

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

type Input struct {
	UserID         *uint64 `json:"userId"`
	EmployeeNumber string  `json:"employeeNumber" binding:"required,max=30"`
	FullName       string  `json:"fullName" binding:"required,max=180"`
	Email          string  `json:"email" binding:"required,email,max=255"`
	Phone          string  `json:"phone" binding:"max=40"`
	BirthDate      string  `json:"birthDate"`
	JoinDate       string  `json:"joinDate" binding:"required"`
	EmploymentType string  `json:"employmentType" binding:"required,max=30"`
	Status         string  `json:"status" binding:"required,oneof=ACTIVE INACTIVE TERMINATED"`
	DepartmentID   *uint64 `json:"departmentId"`
	PositionID     *uint64 `json:"positionId"`
	ManagerID      *uint64 `json:"managerId"`
}

type AvailableUser struct {
	ID    uint64 `json:"id"`
	Email string `json:"email"`
}

func (s *Service) AvailableUsers(ctx context.Context) ([]AvailableUser, error) {
	var users []AvailableUser
	err := s.db.WithContext(ctx).
		Model(&model.User{}).
		Select("users.id, users.email").
		Joins("LEFT JOIN employees ON employees.user_id = users.id AND employees.deleted_at IS NULL").
		Where("users.is_active = ? AND employees.id IS NULL", true).
		Order("users.email ASC").
		Scan(&users).Error
	return users, err
}

func (s *Service) List(ctx context.Context, search, status string, departmentID *uint64, page, limit, offset int) ([]model.Employee, int64, error) {
	query := s.db.WithContext(ctx).Model(&model.Employee{})
	if search != "" {
		term := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(full_name) LIKE ? OR LOWER(employee_number) LIKE ? OR LOWER(email) LIKE ?", term, term, term)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if departmentID != nil {
		query = query.Where("department_id = ?", *departmentID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var employees []model.Employee
	err := query.Preload("Department").Preload("Position").Preload("Manager").Order("full_name ASC").Limit(limit).Offset(offset).Find(&employees).Error
	return employees, total, err
}

func (s *Service) Get(ctx context.Context, id uint64) (model.Employee, error) {
	var item model.Employee
	err := s.db.WithContext(ctx).Preload("Department").Preload("Position").Preload("Manager").First(&item, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return item, httputil.NewDomainError(http.StatusNotFound, "EMPLOYEE_NOT_FOUND", "Employee not found")
	}
	return item, err
}

func (s *Service) Create(ctx context.Context, input Input, actor uint64) (model.Employee, error) {
	item, err := input.toModel()
	if err != nil {
		return model.Employee{}, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateReferences(ctx, tx, &item, 0); err != nil {
			return err
		}
		if err := tx.Create(&item).Error; err != nil {
			if isDuplicate(err) {
				return httputil.NewDomainError(http.StatusConflict, "EMPLOYEE_ALREADY_EXISTS", "Employee number, email, or user is already assigned")
			}
			return err
		}
		return audit.Write(ctx, tx, &actor, "EMPLOYEE_CREATED", "employee", item.ID, map[string]any{"employeeNumber": item.EmployeeNumber})
	})
	if err != nil {
		return model.Employee{}, err
	}
	return s.Get(ctx, item.ID)
}

func (s *Service) Update(ctx context.Context, id uint64, input Input, actor uint64) (model.Employee, error) {
	item, err := input.toModel()
	if err != nil {
		return item, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current model.Employee
		if err := tx.First(&current, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return httputil.NewDomainError(http.StatusNotFound, "EMPLOYEE_NOT_FOUND", "Employee not found")
		} else if err != nil {
			return err
		}
		item.ID = id
		if err := validateReferences(ctx, tx, &item, id); err != nil {
			return err
		}
		updates := map[string]any{"user_id": item.UserID, "employee_number": item.EmployeeNumber, "full_name": item.FullName, "email": item.Email, "phone": item.Phone, "birth_date": item.BirthDate, "join_date": item.JoinDate, "employment_type": item.EmploymentType, "status": item.Status, "department_id": item.DepartmentID, "position_id": item.PositionID, "manager_id": item.ManagerID}
		if err := tx.Model(&current).Updates(updates).Error; err != nil {
			if isDuplicate(err) {
				return httputil.NewDomainError(http.StatusConflict, "EMPLOYEE_ALREADY_EXISTS", "Employee number, email, or user is already assigned")
			}
			return err
		}
		return audit.Write(ctx, tx, &actor, "EMPLOYEE_UPDATED", "employee", id, map[string]any{"employeeNumber": item.EmployeeNumber})
	})
	if err != nil {
		return model.Employee{}, err
	}
	return s.Get(ctx, id)
}

func (i Input) toModel() (model.Employee, error) {
	joinDate, err := time.Parse("2006-01-02", i.JoinDate)
	if err != nil {
		return model.Employee{}, httputil.NewDomainError(http.StatusUnprocessableEntity, "INVALID_JOIN_DATE", "Join date must use YYYY-MM-DD")
	}
	var birthDate *time.Time
	if i.BirthDate != "" {
		parsed, err := time.Parse("2006-01-02", i.BirthDate)
		if err != nil {
			return model.Employee{}, httputil.NewDomainError(http.StatusUnprocessableEntity, "INVALID_BIRTH_DATE", "Birth date must use YYYY-MM-DD")
		}
		birthDate = &parsed
	}
	return model.Employee{UserID: i.UserID, EmployeeNumber: strings.TrimSpace(i.EmployeeNumber), FullName: strings.TrimSpace(i.FullName), Email: strings.ToLower(strings.TrimSpace(i.Email)), Phone: strings.TrimSpace(i.Phone), BirthDate: birthDate, JoinDate: joinDate, EmploymentType: strings.TrimSpace(i.EmploymentType), Status: i.Status, DepartmentID: i.DepartmentID, PositionID: i.PositionID, ManagerID: i.ManagerID}, nil
}

func validateReferences(ctx context.Context, db *gorm.DB, employee *model.Employee, ownID uint64) error {
	if employee.UserID != nil {
		var count int64
		db.WithContext(ctx).Model(&model.User{}).Where("id = ? AND is_active = true", *employee.UserID).Count(&count)
		if count == 0 {
			return httputil.NewDomainError(http.StatusUnprocessableEntity, "INVALID_USER", "Active application user does not exist")
		}
	}
	if employee.ManagerID != nil {
		if *employee.ManagerID == ownID && ownID != 0 {
			return httputil.NewDomainError(http.StatusUnprocessableEntity, "INVALID_MANAGER", "Employee cannot be their own manager")
		}
		var manager model.Employee
		if err := db.WithContext(ctx).First(&manager, *employee.ManagerID).Error; err != nil {
			return httputil.NewDomainError(http.StatusUnprocessableEntity, "INVALID_MANAGER", "Manager does not exist")
		}
		for manager.ManagerID != nil {
			if *manager.ManagerID == ownID {
				return httputil.NewDomainError(http.StatusUnprocessableEntity, "MANAGER_HIERARCHY_CYCLE", "Manager hierarchy cannot contain a cycle")
			}
			if err := db.WithContext(ctx).First(&manager, *manager.ManagerID).Error; err != nil {
				break
			}
		}
	}
	if employee.DepartmentID != nil {
		var count int64
		db.WithContext(ctx).Model(&model.Department{}).Where("id = ? AND is_active = true", *employee.DepartmentID).Count(&count)
		if count == 0 {
			return httputil.NewDomainError(http.StatusUnprocessableEntity, "INVALID_DEPARTMENT", "Active department does not exist")
		}
	}
	if employee.PositionID != nil {
		var count int64
		db.WithContext(ctx).Model(&model.Position{}).Where("id = ? AND is_active = true", *employee.PositionID).Count(&count)
		if count == 0 {
			return httputil.NewDomainError(http.StatusUnprocessableEntity, "INVALID_POSITION", "Active position does not exist")
		}
	}
	return nil
}

func isDuplicate(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "duplicate key")
}
