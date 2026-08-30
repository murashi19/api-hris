package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID           uint64         `gorm:"primaryKey" json:"id"`
	Email        string         `gorm:"size:255;uniqueIndex;not null" json:"email"`
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	IsActive     bool           `gorm:"not null;default:true" json:"isActive"`
	Employee     *Employee      `json:"employee,omitempty"`
	Roles        []Role         `gorm:"many2many:user_roles" json:"roles,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

type Role struct {
	ID          uint64       `gorm:"primaryKey" json:"id"`
	Name        string       `gorm:"size:80;uniqueIndex;not null" json:"name"`
	Description string       `gorm:"size:255" json:"description"`
	Permissions []Permission `gorm:"many2many:role_permissions" json:"permissions,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

type Permission struct {
	ID          uint64    `gorm:"primaryKey" json:"id"`
	Code        string    `gorm:"size:100;uniqueIndex;not null" json:"code"`
	Description string    `gorm:"size:255" json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

type UserRole struct {
	UserID uint64 `gorm:"primaryKey"`
	RoleID uint64 `gorm:"primaryKey"`
}

type RolePermission struct {
	RoleID       uint64 `gorm:"primaryKey"`
	PermissionID uint64 `gorm:"primaryKey"`
}

type RefreshSession struct {
	ID        uint64     `gorm:"primaryKey"`
	UserID    uint64     `gorm:"not null;index"`
	TokenHash string     `gorm:"size:64;uniqueIndex;not null"`
	ExpiresAt time.Time  `gorm:"not null;index"`
	RevokedAt *time.Time `gorm:"index"`
	CreatedAt time.Time
}

type Department struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:120;uniqueIndex;not null" json:"name"`
	IsActive  bool      `gorm:"not null;default:true" json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Position struct {
	ID           uint64      `gorm:"primaryKey" json:"id"`
	Name         string      `gorm:"size:120;not null;uniqueIndex:idx_positions_name_department" json:"name"`
	DepartmentID *uint64     `gorm:"uniqueIndex:idx_positions_name_department" json:"departmentId,omitempty"`
	Department   *Department `json:"department,omitempty"`
	IsActive     bool        `gorm:"not null;default:true" json:"isActive"`
	CreatedAt    time.Time   `json:"createdAt"`
	UpdatedAt    time.Time   `json:"updatedAt"`
}

type Employee struct {
	ID             uint64         `gorm:"primaryKey" json:"id"`
	UserID         *uint64        `gorm:"uniqueIndex" json:"userId,omitempty"`
	EmployeeNumber string         `gorm:"size:30;uniqueIndex;not null" json:"employeeNumber"`
	FullName       string         `gorm:"size:180;not null;index" json:"fullName"`
	Email          string         `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Phone          string         `gorm:"size:40" json:"phone"`
	BirthDate      *time.Time     `gorm:"type:date" json:"birthDate,omitempty"`
	JoinDate       time.Time      `gorm:"type:date;not null" json:"joinDate"`
	EmploymentType string         `gorm:"size:30;not null" json:"employmentType"`
	Status         string         `gorm:"size:20;not null;index" json:"status"`
	DepartmentID   *uint64        `gorm:"index" json:"departmentId,omitempty"`
	Department     *Department    `json:"department,omitempty"`
	PositionID     *uint64        `gorm:"index" json:"positionId,omitempty"`
	Position       *Position      `json:"position,omitempty"`
	ManagerID      *uint64        `gorm:"index" json:"managerId,omitempty"`
	Manager        *Employee      `json:"manager,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

type AttendanceRecord struct {
	ID         uint64     `gorm:"primaryKey" json:"id"`
	EmployeeID uint64     `gorm:"not null;uniqueIndex:idx_attendance_employee_date" json:"employeeId"`
	Employee   *Employee  `json:"employee,omitempty"`
	WorkDate   time.Time  `gorm:"type:date;not null;uniqueIndex:idx_attendance_employee_date;index" json:"workDate"`
	ClockInAt  *time.Time `json:"clockInAt,omitempty"`
	ClockOutAt *time.Time `json:"clockOutAt,omitempty"`
	Status     string     `gorm:"size:20;not null" json:"status"`
	Notes      string     `gorm:"size:500" json:"notes"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type LeaveType struct {
	ID                 uint64    `gorm:"primaryKey" json:"id"`
	Name               string    `gorm:"size:120;uniqueIndex;not null" json:"name"`
	Code               string    `gorm:"size:30;uniqueIndex;not null" json:"code"`
	RequiresBalance    bool      `gorm:"not null;default:true" json:"requiresBalance"`
	RequiresAttachment bool      `gorm:"not null;default:false" json:"requiresAttachment"`
	IsActive           bool      `gorm:"not null;default:true" json:"isActive"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type LeaveBalance struct {
	ID          uint64     `gorm:"primaryKey" json:"id"`
	EmployeeID  uint64     `gorm:"not null;uniqueIndex:idx_leave_balance_owner" json:"employeeId"`
	LeaveTypeID uint64     `gorm:"not null;uniqueIndex:idx_leave_balance_owner" json:"leaveTypeId"`
	LeaveType   *LeaveType `json:"leaveType,omitempty"`
	Year        int        `gorm:"not null;uniqueIndex:idx_leave_balance_owner" json:"year"`
	Entitled    float64    `gorm:"type:numeric(7,2);not null;default:0" json:"entitled"`
	Used        float64    `gorm:"type:numeric(7,2);not null;default:0" json:"used"`
	Adjustment  float64    `gorm:"type:numeric(7,2);not null;default:0" json:"adjustment"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

func (b LeaveBalance) Available() float64 { return b.Entitled + b.Adjustment - b.Used }

type LeaveRequest struct {
	ID          uint64          `gorm:"primaryKey" json:"id"`
	EmployeeID  uint64          `gorm:"not null;index" json:"employeeId"`
	Employee    *Employee       `json:"employee,omitempty"`
	LeaveTypeID uint64          `gorm:"not null;index" json:"leaveTypeId"`
	LeaveType   *LeaveType      `json:"leaveType,omitempty"`
	StartDate   time.Time       `gorm:"type:date;not null;index" json:"startDate"`
	EndDate     time.Time       `gorm:"type:date;not null;index" json:"endDate"`
	Duration    float64         `gorm:"type:numeric(7,2);not null" json:"duration"`
	Reason      string          `gorm:"size:1000;not null" json:"reason"`
	Status      string          `gorm:"size:30;not null;index" json:"status"`
	SubmittedAt time.Time       `gorm:"not null" json:"submittedAt"`
	Approvals   []LeaveApproval `json:"approvals,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type LeaveApproval struct {
	ID             uint64     `gorm:"primaryKey" json:"id"`
	LeaveRequestID uint64     `gorm:"not null;index" json:"leaveRequestId"`
	Stage          string     `gorm:"size:20;not null" json:"stage"`
	Status         string     `gorm:"size:20;not null" json:"status"`
	ActorUserID    *uint64    `gorm:"index" json:"actorUserId,omitempty"`
	Reason         string     `gorm:"size:1000" json:"reason,omitempty"`
	ActedAt        *time.Time `json:"actedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type Notification struct {
	ID        uint64     `gorm:"primaryKey" json:"id"`
	UserID    uint64     `gorm:"not null;index" json:"userId"`
	Type      string     `gorm:"size:60;not null" json:"type"`
	Title     string     `gorm:"size:180;not null" json:"title"`
	Message   string     `gorm:"size:1000;not null" json:"message"`
	ReadAt    *time.Time `gorm:"index" json:"readAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

type AuditLog struct {
	ID          uint64          `gorm:"primaryKey" json:"id"`
	ActorUserID *uint64         `gorm:"index" json:"actorUserId,omitempty"`
	Action      string          `gorm:"size:100;not null;index" json:"action"`
	EntityType  string          `gorm:"size:80;not null;index" json:"entityType"`
	EntityID    string          `gorm:"size:80;not null;index" json:"entityId"`
	Metadata    json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
	CreatedAt   time.Time       `gorm:"index" json:"createdAt"`
}
