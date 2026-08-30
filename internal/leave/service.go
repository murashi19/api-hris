package leave

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hris/backend/internal/audit"
	"hris/backend/internal/httputil"
	"hris/backend/internal/model"
)

const (
	PendingManager = "PENDING_MANAGER"
	PendingHR      = "PENDING_HR"
	Approved       = "APPROVED"
	Rejected       = "REJECTED"
	Cancelled      = "CANCELLED"
)

type Service struct {
	db       *gorm.DB
	location *time.Location
}

func NewService(db *gorm.DB, location *time.Location) *Service {
	return &Service{db: db, location: location}
}

type SubmitInput struct {
	LeaveTypeID uint64 `json:"leaveTypeId" binding:"required"`
	StartDate   string `json:"startDate" binding:"required"`
	EndDate     string `json:"endDate" binding:"required"`
	Reason      string `json:"reason" binding:"required,max=1000"`
}
type LeaveTypeInput struct {
	Name               string `json:"name" binding:"required,max=120"`
	Code               string `json:"code" binding:"required,max=30"`
	RequiresBalance    bool   `json:"requiresBalance"`
	RequiresAttachment bool   `json:"requiresAttachment"`
	IsActive           *bool  `json:"isActive"`
}
type BalanceInput struct {
	EmployeeID  uint64  `json:"employeeId" binding:"required"`
	LeaveTypeID uint64  `json:"leaveTypeId" binding:"required"`
	Year        int     `json:"year" binding:"required,min=2000,max=2200"`
	Entitled    float64 `json:"entitled" binding:"min=0"`
}
type AdjustmentInput struct {
	EmployeeID  uint64  `json:"employeeId" binding:"required"`
	LeaveTypeID uint64  `json:"leaveTypeId" binding:"required"`
	Year        int     `json:"year" binding:"required,min=2000,max=2200"`
	Amount      float64 `json:"amount" binding:"required"`
	Reason      string  `json:"reason" binding:"required,max=1000"`
}

type BalanceView struct {
	model.LeaveBalance
	Available float64 `json:"available"`
}

func (s *Service) Types(ctx context.Context) ([]model.LeaveType, error) {
	var items []model.LeaveType
	err := s.db.WithContext(ctx).Where("is_active=true").Order("name").Find(&items).Error
	return items, err
}
func (s *Service) CreateType(ctx context.Context, in LeaveTypeInput, actor uint64) (model.LeaveType, error) {
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	item := model.LeaveType{Name: strings.TrimSpace(in.Name), Code: strings.ToUpper(strings.TrimSpace(in.Code)), RequiresBalance: in.RequiresBalance, RequiresAttachment: in.RequiresAttachment, IsActive: active}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		return audit.Write(ctx, tx, &actor, "LEAVE_TYPE_CREATED", "leave_type", item.ID, map[string]any{"code": item.Code})
	})
	return item, err
}
func (s *Service) UpdateType(ctx context.Context, id uint64, in LeaveTypeInput, actor uint64) (model.LeaveType, error) {
	var item model.LeaveType
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&item, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return httputil.NewDomainError(404, "LEAVE_TYPE_NOT_FOUND", "Leave type not found")
		} else if err != nil {
			return err
		}
		updates := map[string]any{"name": strings.TrimSpace(in.Name), "code": strings.ToUpper(strings.TrimSpace(in.Code)), "requires_balance": in.RequiresBalance, "requires_attachment": in.RequiresAttachment}
		if in.IsActive != nil {
			updates["is_active"] = *in.IsActive
		}
		if err := tx.Model(&item).Updates(updates).Error; err != nil {
			return err
		}
		return audit.Write(ctx, tx, &actor, "LEAVE_TYPE_UPDATED", "leave_type", id, updates)
	})
	return item, err
}
func (s *Service) SetEntitlement(ctx context.Context, in BalanceInput, actor uint64) (model.LeaveBalance, error) {
	var balance model.LeaveBalance
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateBalanceReferences(tx, in.EmployeeID, in.LeaveTypeID); err != nil {
			return err
		}
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("employee_id=? AND leave_type_id=? AND year=?", in.EmployeeID, in.LeaveTypeID, in.Year).First(&balance).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			balance = model.LeaveBalance{EmployeeID: in.EmployeeID, LeaveTypeID: in.LeaveTypeID, Year: in.Year, Entitled: in.Entitled}
			if err := tx.Create(&balance).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if err := tx.Model(&balance).Update("entitled", in.Entitled).Error; err != nil {
			return err
		}
		return audit.Write(ctx, tx, &actor, "LEAVE_BALANCE_ENTITLEMENT_SET", "leave_balance", balance.ID, map[string]any{"entitled": in.Entitled, "year": in.Year})
	})
	return balance, err
}
func (s *Service) AdjustBalance(ctx context.Context, in AdjustmentInput, actor uint64) (model.LeaveBalance, error) {
	var balance model.LeaveBalance
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateBalanceReferences(tx, in.EmployeeID, in.LeaveTypeID); err != nil {
			return err
		}
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("employee_id=? AND leave_type_id=? AND year=?", in.EmployeeID, in.LeaveTypeID, in.Year).First(&balance).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return httputil.NewDomainError(404, "LEAVE_BALANCE_NOT_FOUND", "Set entitlement before adjusting a leave balance")
		} else if err != nil {
			return err
		}
		if balance.Available()+in.Amount < 0 {
			return httputil.NewDomainError(422, "INVALID_LEAVE_ADJUSTMENT", "Adjustment cannot make available balance negative")
		}
		if err := tx.Model(&balance).Update("adjustment", gorm.Expr("adjustment + ?", in.Amount)).Error; err != nil {
			return err
		}
		balance.Adjustment += in.Amount
		return audit.Write(ctx, tx, &actor, "LEAVE_BALANCE_ADJUSTED", "leave_balance", balance.ID, map[string]any{"amount": in.Amount, "reason": in.Reason})
	})
	return balance, err
}
func validateBalanceReferences(tx *gorm.DB, employeeID, leaveTypeID uint64) error {
	var count int64
	if err := tx.Model(&model.Employee{}).Where("id=?", employeeID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return httputil.NewDomainError(422, "INVALID_EMPLOYEE", "Employee does not exist")
	}
	if err := tx.Model(&model.LeaveType{}).Where("id=?", leaveTypeID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return httputil.NewDomainError(422, "INVALID_LEAVE_TYPE", "Leave type does not exist")
	}
	return nil
}
func (s *Service) Balances(ctx context.Context, employeeID uint64) ([]BalanceView, error) {
	var balances []model.LeaveBalance
	err := s.db.WithContext(ctx).Preload("LeaveType").Where("employee_id=? AND year=?", employeeID, time.Now().In(s.location).Year()).Order("leave_type_id").Find(&balances).Error
	if err != nil {
		return nil, err
	}
	items := make([]BalanceView, 0, len(balances))
	for _, b := range balances {
		items = append(items, BalanceView{LeaveBalance: b, Available: b.Available()})
	}
	return items, nil
}

func (s *Service) Submit(ctx context.Context, employeeID, userID uint64, in SubmitInput) (model.LeaveRequest, error) {
	start, end, duration, err := parseDateRange(in.StartDate, in.EndDate, s.location)
	if err != nil {
		return model.LeaveRequest{}, err
	}
	var request model.LeaveRequest
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var employee model.Employee
		if err := tx.First(&employee, employeeID).Error; err != nil {
			return err
		}
		if employee.ManagerID == nil {
			return httputil.NewDomainError(422, "MANAGER_NOT_CONFIGURED", "A manager must be configured before submitting leave")
		}
		var manager model.Employee
		if err := tx.First(&manager, *employee.ManagerID).Error; err != nil || manager.UserID == nil {
			return httputil.NewDomainError(422, "MANAGER_ACCOUNT_NOT_CONFIGURED", "The employee manager must have a user account")
		}
		var leaveType model.LeaveType
		if err := tx.Where("id=? AND is_active=true", in.LeaveTypeID).First(&leaveType).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return httputil.NewDomainError(422, "INVALID_LEAVE_TYPE", "Active leave type does not exist")
		} else if err != nil {
			return err
		}
		if leaveType.RequiresAttachment {
			return httputil.NewDomainError(422, "ATTACHMENT_REQUIRED", "This leave type requires an attachment; attachment workflow is not enabled in this MVP")
		}
		var overlap int64
		if err := tx.Model(&model.LeaveRequest{}).Where("employee_id=? AND status IN ? AND start_date<=? AND end_date>=?", employeeID, []string{PendingManager, PendingHR, Approved}, end, start).Count(&overlap).Error; err != nil {
			return err
		}
		if overlap > 0 {
			return httputil.NewDomainError(http.StatusConflict, "OVERLAPPING_LEAVE_REQUEST", "Leave dates overlap an active request")
		}
		if leaveType.RequiresBalance {
			var balance model.LeaveBalance
			if err := tx.Where("employee_id=? AND leave_type_id=? AND year=?", employeeID, in.LeaveTypeID, start.Year()).First(&balance).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return httputil.NewDomainError(422, "LEAVE_BALANCE_NOT_FOUND", "Leave balance is not configured for this year")
			} else if err != nil {
				return err
			}
			if balance.Available() < duration {
				return httputil.NewDomainError(422, "INSUFFICIENT_LEAVE_BALANCE", "Insufficient leave balance")
			}
		}
		now := time.Now().In(s.location)
		reason := strings.TrimSpace(in.Reason)
		if reason == "" {
			return httputil.NewDomainError(422, "LEAVE_REASON_REQUIRED", "Leave reason is required")
		}
		request = model.LeaveRequest{EmployeeID: employeeID, LeaveTypeID: in.LeaveTypeID, StartDate: start, EndDate: end, Duration: duration, Reason: reason, Status: PendingManager, SubmittedAt: now}
		if err := tx.Create(&request).Error; err != nil {
			return err
		}
		approval := model.LeaveApproval{LeaveRequestID: request.ID, Stage: "MANAGER", Status: "PENDING"}
		if err := tx.Create(&approval).Error; err != nil {
			return err
		}
		if err := createNotification(ctx, tx, *manager.UserID, "LEAVE_SUBMITTED", "Leave request awaiting approval", fmt.Sprintf("%s submitted a leave request", employee.FullName)); err != nil {
			return err
		}
		return audit.Write(ctx, tx, &userID, "LEAVE_SUBMITTED", "leave_request", request.ID, map[string]any{"startDate": in.StartDate, "endDate": in.EndDate, "duration": duration})
	})
	if err != nil {
		return model.LeaveRequest{}, err
	}
	return s.Get(ctx, request.ID, userID, employeeID, true)
}

func (s *Service) Mine(ctx context.Context, employeeID uint64, page, limit, offset int) ([]model.LeaveRequest, int64, error) {
	q := s.db.WithContext(ctx).Model(&model.LeaveRequest{}).Where("employee_id=?", employeeID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.LeaveRequest
	err := q.Preload("LeaveType").Order("submitted_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}
func (s *Service) Get(ctx context.Context, id, userID, employeeID uint64, canReadAll bool) (model.LeaveRequest, error) {
	var item model.LeaveRequest
	err := s.db.WithContext(ctx).Preload("Employee").Preload("LeaveType").Preload("Approvals").First(&item, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return item, httputil.NewDomainError(404, "LEAVE_REQUEST_NOT_FOUND", "Leave request not found")
	} else if err != nil {
		return item, err
	}
	if !canReadAll && item.EmployeeID != employeeID {
		var employee model.Employee
		if err := s.db.WithContext(ctx).First(&employee, item.EmployeeID).Error; err != nil || employee.ManagerID == nil || *employee.ManagerID != employeeID {
			return item, httputil.NewDomainError(403, "FORBIDDEN", "Leave request is outside your scope")
		}
	}
	return item, nil
}

func (s *Service) ApprovalQueue(ctx context.Context, actorEmployeeID uint64, hr bool, page, limit, offset int) ([]model.LeaveRequest, int64, error) {
	q := s.db.WithContext(ctx).Model(&model.LeaveRequest{})
	if hr {
		q = q.Where("status=?", PendingHR)
	} else {
		q = q.Joins("JOIN employees e ON e.id=leave_requests.employee_id").Where("leave_requests.status=? AND e.manager_id=?", PendingManager, actorEmployeeID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.LeaveRequest
	err := q.Preload("Employee").Preload("LeaveType").Order("submitted_at").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) ManagerApprove(ctx context.Context, id, userID, managerEmployeeID uint64) (model.LeaveRequest, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var req model.LeaveRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&req, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return httputil.NewDomainError(404, "LEAVE_REQUEST_NOT_FOUND", "Leave request not found")
		} else if err != nil {
			return err
		}
		if req.Status != PendingManager {
			return httputil.NewDomainError(409, "INVALID_LEAVE_STATUS", "Only requests pending manager approval can be approved")
		}
		var employee model.Employee
		if err := tx.First(&employee, req.EmployeeID).Error; err != nil {
			return err
		}
		if employee.ManagerID == nil || *employee.ManagerID != managerEmployeeID {
			return httputil.NewDomainError(403, "LEAVE_OUTSIDE_MANAGER_SCOPE", "Only the direct manager can approve this request")
		}
		now := time.Now().In(s.location)
		result := tx.Model(&model.LeaveApproval{}).Where("leave_request_id=? AND stage='MANAGER' AND status='PENDING'", id).Updates(map[string]any{"status": "APPROVED", "actor_user_id": userID, "acted_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return httputil.NewDomainError(409, "APPROVAL_ALREADY_PROCESSED", "Approval has already been processed")
		}
		if err := tx.Model(&req).Update("status", PendingHR).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.LeaveApproval{LeaveRequestID: id, Stage: "HR", Status: "PENDING"}).Error; err != nil {
			return err
		}
		if err := notifyPermission(ctx, tx, "leave.hr.approve", "LEAVE_MANAGER_APPROVED", "Leave request awaiting HR approval", fmt.Sprintf("Leave request #%d was approved by the manager", id)); err != nil {
			return err
		}
		return audit.Write(ctx, tx, &userID, "LEAVE_MANAGER_APPROVED", "leave_request", id, map[string]any{})
	})
	if err != nil {
		return model.LeaveRequest{}, err
	}
	return s.Get(ctx, id, userID, managerEmployeeID, true)
}

func (s *Service) HRApprove(ctx context.Context, id, userID uint64) (model.LeaveRequest, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var req model.LeaveRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&req, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return httputil.NewDomainError(404, "LEAVE_REQUEST_NOT_FOUND", "Leave request not found")
		} else if err != nil {
			return err
		}
		if req.Status != PendingHR {
			return httputil.NewDomainError(409, "INVALID_LEAVE_STATUS", "Only requests pending HR approval can be approved")
		}
		var leaveType model.LeaveType
		if err := tx.First(&leaveType, req.LeaveTypeID).Error; err != nil {
			return err
		}
		if leaveType.RequiresBalance {
			var balance model.LeaveBalance
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("employee_id=? AND leave_type_id=? AND year=?", req.EmployeeID, req.LeaveTypeID, req.StartDate.Year()).First(&balance).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return httputil.NewDomainError(422, "LEAVE_BALANCE_NOT_FOUND", "Leave balance is not configured")
			} else if err != nil {
				return err
			}
			if balance.Available() < req.Duration {
				return httputil.NewDomainError(422, "INSUFFICIENT_LEAVE_BALANCE", "Insufficient leave balance")
			}
			if err := tx.Model(&balance).Update("used", gorm.Expr("used + ?", req.Duration)).Error; err != nil {
				return err
			}
		}
		now := time.Now().In(s.location)
		result := tx.Model(&model.LeaveApproval{}).Where("leave_request_id=? AND stage='HR' AND status='PENDING'", id).Updates(map[string]any{"status": "APPROVED", "actor_user_id": userID, "acted_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return httputil.NewDomainError(409, "APPROVAL_ALREADY_PROCESSED", "Approval has already been processed")
		}
		if err := tx.Model(&req).Update("status", Approved).Error; err != nil {
			return err
		}
		if err := notifyEmployee(ctx, tx, req.EmployeeID, "LEAVE_APPROVED", "Leave request approved", fmt.Sprintf("Your leave request #%d was approved", id)); err != nil {
			return err
		}
		return audit.Write(ctx, tx, &userID, "LEAVE_APPROVED", "leave_request", id, map[string]any{"duration": req.Duration})
	})
	if err != nil {
		return model.LeaveRequest{}, err
	}
	return s.Get(ctx, id, userID, 0, true)
}

func (s *Service) Reject(ctx context.Context, id, userID, actorEmployeeID uint64, isHR bool, reason string) (model.LeaveRequest, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return model.LeaveRequest{}, httputil.NewDomainError(422, "REJECTION_REASON_REQUIRED", "Rejection reason is required")
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var req model.LeaveRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&req, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return httputil.NewDomainError(404, "LEAVE_REQUEST_NOT_FOUND", "Leave request not found")
		} else if err != nil {
			return err
		}
		stage := ""
		if req.Status == PendingManager {
			stage = "MANAGER"
			var employee model.Employee
			if err := tx.First(&employee, req.EmployeeID).Error; err != nil {
				return err
			}
			if employee.ManagerID == nil || *employee.ManagerID != actorEmployeeID {
				return httputil.NewDomainError(403, "LEAVE_OUTSIDE_MANAGER_SCOPE", "Only the direct manager can reject this request")
			}
		} else if req.Status == PendingHR {
			stage = "HR"
			if !isHR {
				return httputil.NewDomainError(403, "FORBIDDEN", "HR permission is required at this approval stage")
			}
		} else {
			return httputil.NewDomainError(409, "INVALID_LEAVE_STATUS", "This request can no longer be rejected")
		}
		now := time.Now().In(s.location)
		result := tx.Model(&model.LeaveApproval{}).Where("leave_request_id=? AND stage=? AND status='PENDING'", id, stage).Updates(map[string]any{"status": "REJECTED", "actor_user_id": userID, "reason": reason, "acted_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return httputil.NewDomainError(409, "APPROVAL_ALREADY_PROCESSED", "Approval has already been processed")
		}
		if err := tx.Model(&req).Update("status", Rejected).Error; err != nil {
			return err
		}
		if err := notifyEmployee(ctx, tx, req.EmployeeID, "LEAVE_REJECTED", "Leave request rejected", fmt.Sprintf("Your leave request #%d was rejected: %s", id, reason)); err != nil {
			return err
		}
		return audit.Write(ctx, tx, &userID, "LEAVE_REJECTED", "leave_request", id, map[string]any{"stage": stage, "reason": reason})
	})
	if err != nil {
		return model.LeaveRequest{}, err
	}
	return s.Get(ctx, id, userID, actorEmployeeID, true)
}

func (s *Service) Cancel(ctx context.Context, id, userID, employeeID uint64) (model.LeaveRequest, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var req model.LeaveRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&req, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return httputil.NewDomainError(404, "LEAVE_REQUEST_NOT_FOUND", "Leave request not found")
		} else if err != nil {
			return err
		}
		if req.EmployeeID != employeeID {
			return httputil.NewDomainError(403, "FORBIDDEN", "You can only cancel your own request")
		}
		if req.Status != PendingManager && req.Status != PendingHR {
			return httputil.NewDomainError(409, "LEAVE_CANNOT_BE_CANCELLED", "Only pending leave requests can be cancelled")
		}
		if err := tx.Model(&req).Update("status", Cancelled).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.LeaveApproval{}).Where("leave_request_id=? AND status='PENDING'", id).Update("status", "CANCELLED").Error; err != nil {
			return err
		}
		return audit.Write(ctx, tx, &userID, "LEAVE_CANCELLED", "leave_request", id, map[string]any{})
	})
	if err != nil {
		return model.LeaveRequest{}, err
	}
	return s.Get(ctx, id, userID, employeeID, true)
}

func createNotification(ctx context.Context, tx *gorm.DB, userID uint64, kind, title, message string) error {
	return tx.WithContext(ctx).Create(&model.Notification{UserID: userID, Type: kind, Title: title, Message: message}).Error
}
func notifyEmployee(ctx context.Context, tx *gorm.DB, employeeID uint64, kind, title, message string) error {
	var employee model.Employee
	if err := tx.First(&employee, employeeID).Error; err != nil {
		return err
	}
	if employee.UserID == nil {
		return nil
	}
	return createNotification(ctx, tx, *employee.UserID, kind, title, message)
}
func notifyPermission(ctx context.Context, tx *gorm.DB, permission, kind, title, message string) error {
	var ids []uint64
	if err := tx.Table("user_roles ur").Distinct("ur.user_id").Joins("JOIN role_permissions rp ON rp.role_id=ur.role_id").Joins("JOIN permissions p ON p.id=rp.permission_id").Where("p.code=?", permission).Pluck("ur.user_id", &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := createNotification(ctx, tx, id, kind, title, message); err != nil {
			return err
		}
	}
	return nil
}
