package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"hris/backend/internal/httputil"
	"hris/backend/internal/model"
	"hris/backend/internal/security"
)

const (
	ContextUserID      = "auth_user_id"
	ContextEmployeeID  = "auth_employee_id"
	ContextPermissions = "auth_permissions"
)

func Authenticate(db *gorm.DB, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			httputil.Error(c, http.StatusUnauthorized, "Authentication is required", "UNAUTHORIZED")
			return
		}
		userID, err := security.ParseAccessToken(parts[1], jwtSecret)
		if err != nil {
			httputil.Error(c, http.StatusUnauthorized, "Invalid or expired access token", "INVALID_ACCESS_TOKEN")
			return
		}
		var user model.User
		if err := db.WithContext(c.Request.Context()).Preload("Employee").First(&user, userID).Error; err != nil || !user.IsActive {
			httputil.Error(c, http.StatusUnauthorized, "Account is unavailable", "ACCOUNT_UNAVAILABLE")
			return
		}
		var codes []string
		db.WithContext(c.Request.Context()).Table("permissions p").Distinct("p.code").
			Joins("JOIN role_permissions rp ON rp.permission_id = p.id").
			Joins("JOIN user_roles ur ON ur.role_id = rp.role_id").
			Where("ur.user_id = ?", user.ID).Pluck("p.code", &codes)
		permissions := make(map[string]struct{}, len(codes))
		for _, code := range codes {
			permissions[code] = struct{}{}
		}
		c.Set(ContextUserID, user.ID)
		if user.Employee != nil {
			c.Set(ContextEmployeeID, user.Employee.ID)
		}
		c.Set(ContextPermissions, permissions)
		c.Next()
	}
}

func RequirePermission(code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		permissions, ok := c.Get(ContextPermissions)
		if !ok {
			httputil.Error(c, http.StatusUnauthorized, "Authentication is required", "UNAUTHORIZED")
			return
		}
		if _, allowed := permissions.(map[string]struct{})[code]; !allowed {
			httputil.Error(c, http.StatusForbidden, "You do not have permission to perform this action", "FORBIDDEN")
			return
		}
		c.Next()
	}
}

func RequireAnyPermission(codes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, ok := c.Get(ContextPermissions)
		if !ok {
			httputil.Error(c, http.StatusUnauthorized, "Authentication is required", "UNAUTHORIZED")
			return
		}
		permissions := value.(map[string]struct{})
		for _, code := range codes {
			if _, allowed := permissions[code]; allowed {
				c.Next()
				return
			}
		}
		httputil.Error(c, http.StatusForbidden, "You do not have permission to perform this action", "FORBIDDEN")
	}
}

func HasPermission(c *gin.Context, code string) bool {
	value, ok := c.Get(ContextPermissions)
	if !ok {
		return false
	}
	_, allowed := value.(map[string]struct{})[code]
	return allowed
}

func UserID(c *gin.Context) uint64 {
	value, _ := c.Get(ContextUserID)
	id, _ := value.(uint64)
	return id
}

func EmployeeID(c *gin.Context) (uint64, bool) {
	value, ok := c.Get(ContextEmployeeID)
	if !ok {
		return 0, false
	}
	id, ok := value.(uint64)
	return id, ok
}
