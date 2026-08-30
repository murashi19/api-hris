package attendance

import (
	"context"
	"errors"
	"net/http"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hris/backend/internal/audit"
	"hris/backend/internal/httputil"
	"hris/backend/internal/model"
)

type Service struct {
	db       *gorm.DB
	location *time.Location
}

func NewService(db *gorm.DB, location *time.Location) *Service {
	return &Service{db: db, location: location}
}

func (s *Service) ClockIn(ctx context.Context, employeeID, userID uint64, notes string) (model.AttendanceRecord, error) {
	now := time.Now().In(s.location)
	workDate := dateOnly(now)
	record := model.AttendanceRecord{EmployeeID: employeeID, WorkDate: workDate, ClockInAt: &now, Status: "PRESENT", Notes: notes}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.AttendanceRecord{}).Where("employee_id=? AND work_date=?", employeeID, workDate).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return httputil.NewDomainError(http.StatusConflict, "ALREADY_CLOCKED_IN", "Attendance already exists for this work date")
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		return audit.Write(ctx, tx, &userID, "ATTENDANCE_CLOCKED_IN", "attendance_record", record.ID, map[string]any{"workDate": workDate.Format("2006-01-02")})
	})
	return record, err
}

func (s *Service) ClockOut(ctx context.Context, employeeID, userID uint64) (model.AttendanceRecord, error) {
	now := time.Now().In(s.location)
	workDate := dateOnly(now)
	var record model.AttendanceRecord
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("employee_id=? AND work_date=?", employeeID, workDate).First(&record).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return httputil.NewDomainError(http.StatusConflict, "CLOCK_IN_REQUIRED", "Clock-in is required before clock-out")
		} else if err != nil {
			return err
		}
		if record.ClockOutAt != nil {
			return httputil.NewDomainError(http.StatusConflict, "ALREADY_CLOCKED_OUT", "Attendance has already been clocked out")
		}
		if record.ClockInAt == nil || now.Before(*record.ClockInAt) {
			return httputil.NewDomainError(422, "INVALID_CLOCK_OUT", "Clock-out cannot be earlier than clock-in")
		}
		if err := tx.Model(&record).Update("clock_out_at", now).Error; err != nil {
			return err
		}
		record.ClockOutAt = &now
		return audit.Write(ctx, tx, &userID, "ATTENDANCE_CLOCKED_OUT", "attendance_record", record.ID, map[string]any{"workDate": workDate.Format("2006-01-02")})
	})
	return record, err
}

func (s *Service) ListMine(ctx context.Context, employeeID uint64, page, limit, offset int) ([]model.AttendanceRecord, int64, error) {
	return s.list(ctx, employeeID, nil, nil, page, limit, offset)
}
func (s *Service) ListAll(ctx context.Context, employeeID *uint64, start, end *time.Time, page, limit, offset int) ([]model.AttendanceRecord, int64, error) {
	id := uint64(0)
	if employeeID != nil {
		id = *employeeID
	}
	return s.list(ctx, id, start, end, page, limit, offset)
}

func (s *Service) ListTeam(ctx context.Context, managerID uint64, start, end *time.Time, page, limit, offset int) ([]model.AttendanceRecord, int64, error) {
	query := s.db.WithContext(ctx).Model(&model.AttendanceRecord{}).
		Joins("JOIN employees team_employee ON team_employee.id = attendance_records.employee_id").
		Where("team_employee.manager_id = ?", managerID)
	if start != nil {
		query = query.Where("attendance_records.work_date >= ?", *start)
	}
	if end != nil {
		query = query.Where("attendance_records.work_date <= ?", *end)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.AttendanceRecord
	err := query.Preload("Employee").Order("attendance_records.work_date DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}
func (s *Service) list(ctx context.Context, employeeID uint64, start, end *time.Time, page, limit, offset int) ([]model.AttendanceRecord, int64, error) {
	q := s.db.WithContext(ctx).Model(&model.AttendanceRecord{})
	if employeeID > 0 {
		q = q.Where("employee_id=?", employeeID)
	}
	if start != nil {
		q = q.Where("work_date>=?", *start)
	}
	if end != nil {
		q = q.Where("work_date<=?", *end)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.AttendanceRecord
	err := q.Preload("Employee").Order("work_date DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}
func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
