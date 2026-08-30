package app

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"hris/backend/internal/attendance"
	"hris/backend/internal/audit"
	"hris/backend/internal/auth"
	"hris/backend/internal/config"
	"hris/backend/internal/department"
	"hris/backend/internal/employee"
	"hris/backend/internal/httputil"
	"hris/backend/internal/leave"
	"hris/backend/internal/middleware"
	"hris/backend/internal/notification"
	"hris/backend/internal/position"
	"hris/backend/internal/rbac"
)

func New(cfg config.Config, db *gorm.DB, redisClient *redis.Client, log *slog.Logger) *gin.Engine {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(middleware.Recovery(log), middleware.Logger(log), middleware.SecurityHeaders())
	router.Use(cors.New(cors.Config{AllowOrigins: cfg.CORSOrigins, AllowMethods: []string{"GET", "POST", "PATCH", "PUT", "OPTIONS"}, AllowHeaders: []string{"Authorization", "Content-Type"}, AllowCredentials: true, MaxAge: 12 * time.Hour}))
	router.GET("/health", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
			httputil.Error(c, 503, "Database is unavailable", "DATABASE_UNAVAILABLE")
			return
		}
		httputil.OK(c, http.StatusOK, "Service is healthy", map[string]string{"status": "ok"})
	})

	authService := auth.NewService(db, cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	authHandler := auth.NewHandler(authService, cfg)
	api := router.Group("/api")
	authRoutes := api.Group("/auth")
	authRoutes.POST("/login", middleware.RateLimit(redisClient, "login", 10, time.Minute), authHandler.Login)
	authRoutes.POST("/refresh", middleware.RateLimit(redisClient, "refresh", 30, time.Minute), authHandler.Refresh)
	authRoutes.POST("/logout", middleware.RateLimit(redisClient, "logout", 30, time.Minute), authHandler.Logout)
	authenticated := api.Group("")
	authenticated.Use(middleware.Authenticate(db, cfg.JWTSecret))
	authenticated.GET("/auth/me", authHandler.Me)

	employeeHandler := employee.NewHandler(employee.NewService(db))
	employees := authenticated.Group("/employees")
	employees.GET("", middleware.RequirePermission("employee.read"), employeeHandler.List)
	employees.POST("", middleware.RequirePermission("employee.create"), employeeHandler.Create)
	employees.GET("/:id", middleware.RequirePermission("employee.read"), employeeHandler.Get)
	employees.PATCH("/:id", middleware.RequirePermission("employee.update"), employeeHandler.Update)
	departmentHandler := department.NewHandler(department.NewService(db))
	departments := authenticated.Group("/departments")
	departments.GET("", middleware.RequirePermission("department.read"), departmentHandler.List)
	departments.POST("", middleware.RequirePermission("department.manage"), departmentHandler.Create)
	departments.PATCH("/:id", middleware.RequirePermission("department.manage"), departmentHandler.Update)
	positionHandler := position.NewHandler(position.NewService(db))
	positions := authenticated.Group("/positions")
	positions.GET("", middleware.RequirePermission("position.read"), positionHandler.List)
	positions.POST("", middleware.RequirePermission("position.manage"), positionHandler.Create)
	positions.PATCH("/:id", middleware.RequirePermission("position.manage"), positionHandler.Update)

	location, _ := time.LoadLocation(cfg.Timezone)
	attendanceHandler := attendance.NewHandler(attendance.NewService(db, location))
	attendanceRoutes := authenticated.Group("/attendance")
	attendanceRoutes.GET("/me", middleware.RequirePermission("attendance.self.read"), attendanceHandler.Mine)
	attendanceRoutes.GET("/team", middleware.RequirePermission("attendance.team.read"), attendanceHandler.Team)
	attendanceRoutes.POST("/clock-in", middleware.RequirePermission("attendance.clock"), attendanceHandler.ClockIn)
	attendanceRoutes.POST("/clock-out", middleware.RequirePermission("attendance.clock"), attendanceHandler.ClockOut)
	attendanceRoutes.GET("", middleware.RequirePermission("attendance.all.read"), attendanceHandler.All)

	leaveHandler := leave.NewHandler(leave.NewService(db, location))
	authenticated.GET("/leave-types", leaveHandler.Types)
	authenticated.POST("/leave-types", middleware.RequirePermission("leave.type.manage"), leaveHandler.CreateType)
	authenticated.PATCH("/leave-types/:id", middleware.RequirePermission("leave.type.manage"), leaveHandler.UpdateType)
	authenticated.GET("/leave-balances/me", middleware.RequirePermission("leave.self.read"), leaveHandler.Balances)
	authenticated.PUT("/leave-balances/entitlement", middleware.RequirePermission("leave.balance.manage"), leaveHandler.SetEntitlement)
	authenticated.POST("/leave-balances/adjust", middleware.RequirePermission("leave.balance.manage"), leaveHandler.AdjustBalance)
	authenticated.GET("/leave-requests/me", middleware.RequirePermission("leave.self.read"), leaveHandler.Mine)
	authenticated.POST("/leave-requests", middleware.RequirePermission("leave.create"), leaveHandler.Submit)
	authenticated.GET("/leave-requests/:id", middleware.RequireAnyPermission("leave.self.read", "leave.team.read", "leave.all.read"), leaveHandler.Get)
	authenticated.POST("/leave-requests/:id/cancel", middleware.RequirePermission("leave.create"), leaveHandler.Cancel)
	authenticated.GET("/leave-approvals", middleware.RequireAnyPermission("leave.manager.approve", "leave.hr.approve"), leaveHandler.Approvals)
	authenticated.POST("/leave-requests/:id/manager-approve", middleware.RequirePermission("leave.manager.approve"), leaveHandler.ManagerApprove)
	authenticated.POST("/leave-requests/:id/hr-approve", middleware.RequirePermission("leave.hr.approve"), leaveHandler.HRApprove)
	authenticated.POST("/leave-requests/:id/reject", middleware.RequireAnyPermission("leave.manager.approve", "leave.hr.approve"), leaveHandler.Reject)

	notificationHandler := notification.NewHandler(db)
	authenticated.GET("/notifications", notificationHandler.List)
	authenticated.POST("/notifications/:id/read", notificationHandler.Read)
	auditHandler := audit.NewHandler(db)
	authenticated.GET("/audit-logs", middleware.RequirePermission("audit.read"), auditHandler.List)
	rbacHandler := rbac.NewHandler(rbac.NewService(db))
	admin := authenticated.Group("/admin")
	admin.Use(middleware.RequirePermission("role.manage"))
	admin.GET("/users", rbacHandler.Users)
	admin.POST("/users", rbacHandler.CreateUser)
	admin.PUT("/users/:id/roles", rbacHandler.AssignRoles)
	admin.GET("/roles", rbacHandler.Roles)
	admin.POST("/roles", rbacHandler.CreateRole)
	admin.GET("/permissions", rbacHandler.Permissions)
	return router
}
