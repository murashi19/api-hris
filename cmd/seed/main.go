package main

import (
	"log/slog"
	"os"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hris/backend/internal/config"
	"hris/backend/internal/database"
	"hris/backend/internal/model"
	"hris/backend/internal/security"
)

var permissionDescriptions = map[string]string{
	"employee.read": "View employees", "employee.create": "Create employees", "employee.update": "Update employees", "employee.delete": "Deactivate employees",
	"department.read": "View departments", "department.manage": "Manage departments", "position.read": "View positions", "position.manage": "Manage positions",
	"attendance.self.read": "View own attendance", "attendance.team.read": "View team attendance", "attendance.all.read": "View all attendance", "attendance.clock": "Clock in and out",
	"leave.self.read": "View own leave", "leave.create": "Submit and cancel leave", "leave.team.read": "View team leave", "leave.all.read": "View all leave", "leave.manager.approve": "Manager leave approval", "leave.hr.approve": "HR leave approval", "leave.type.manage": "Manage leave types", "leave.balance.manage": "Manage leave balances",
	"role.manage": "Manage users, roles, and permissions", "audit.read": "View audit logs",
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		log.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	password := os.Getenv("SEED_ADMIN_PASSWORD")
	if len(password) < 12 {
		log.Error("SEED_ADMIN_PASSWORD must contain at least 12 characters")
		os.Exit(1)
	}
	db, err := database.Open(cfg.DatabaseURL, cfg.Environment == "production")
	if err != nil {
		log.Error("database failed", "error", err)
		os.Exit(1)
	}
	if err := seed(db, password); err != nil {
		log.Error("seed failed", "error", err)
		os.Exit(1)
	}
	log.Info("seed completed")
}

func seed(db *gorm.DB, password string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		permissions := map[string]model.Permission{}
		for code, description := range permissionDescriptions {
			item := model.Permission{Code: code, Description: description}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "code"}}, DoUpdates: clause.AssignmentColumns([]string{"description"})}).Create(&item).Error; err != nil {
				return err
			}
			if err := tx.Where("code=?", code).First(&item).Error; err != nil {
				return err
			}
			permissions[code] = item
		}
		rolePermissions := map[string][]string{"Super Admin": keys(permissionDescriptions), "HR": {"employee.read", "employee.create", "employee.update", "department.read", "department.manage", "position.read", "position.manage", "attendance.all.read", "leave.self.read", "leave.create", "leave.all.read", "leave.hr.approve", "leave.type.manage", "leave.balance.manage", "audit.read"}, "Manager": {"department.read", "position.read", "attendance.self.read", "attendance.team.read", "attendance.clock", "leave.self.read", "leave.create", "leave.team.read", "leave.manager.approve"}, "Employee": {"attendance.self.read", "attendance.clock", "leave.self.read", "leave.create"}}
		var superAdmin model.Role
		for name, codes := range rolePermissions {
			role := model.Role{Name: name, Description: name + " application role"}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoUpdates: clause.AssignmentColumns([]string{"description"})}).Create(&role).Error; err != nil {
				return err
			}
			if err := tx.Where("name=?", name).First(&role).Error; err != nil {
				return err
			}
			assigned := make([]model.Permission, 0, len(codes))
			for _, code := range codes {
				assigned = append(assigned, permissions[code])
			}
			if err := tx.Model(&role).Association("Permissions").Replace(assigned); err != nil {
				return err
			}
			if name == "Super Admin" {
				superAdmin = role
			}
		}
		email := strings.ToLower(strings.TrimSpace(os.Getenv("SEED_ADMIN_EMAIL")))
		if email == "" {
			email = "admin@hris.local"
		}
		hash, err := security.HashPassword(password)
		if err != nil {
			return err
		}
		user := model.User{Email: email, PasswordHash: hash, IsActive: true}
		if err := tx.Where("email=?", email).FirstOrCreate(&user).Error; err != nil {
			return err
		}
		if err := tx.Model(&user).Updates(map[string]any{"password_hash": hash, "is_active": true}).Error; err != nil {
			return err
		}
		if err := tx.Model(&user).Association("Roles").Replace([]model.Role{superAdmin}); err != nil {
			return err
		}
		for _, name := range []string{"Engineering", "Human Resources", "Finance", "Marketing", "Operations"} {
			department := model.Department{Name: name, IsActive: true}
			if err := tx.Where("name=?", name).FirstOrCreate(&department).Error; err != nil {
				return err
			}
		}
		for _, item := range []model.LeaveType{{Name: "Annual Leave", Code: "ANNUAL", RequiresBalance: true, IsActive: true}, {Name: "Sick Leave", Code: "SICK", RequiresBalance: false, IsActive: true}, {Name: "Unpaid Leave", Code: "UNPAID", RequiresBalance: false, IsActive: true}} {
			if err := tx.Exec(`
				INSERT INTO leave_types (name, code, requires_balance, requires_attachment, is_active)
				VALUES (?, ?, ?, ?, ?)
				ON CONFLICT (code) DO UPDATE SET
					name = EXCLUDED.name,
					requires_balance = EXCLUDED.requires_balance,
					requires_attachment = EXCLUDED.requires_attachment,
					is_active = EXCLUDED.is_active,
					updated_at = NOW()`,
				item.Name, item.Code, item.RequiresBalance, item.RequiresAttachment, item.IsActive,
			).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
func keys(values map[string]string) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	return result
}
