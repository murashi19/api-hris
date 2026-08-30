package rbac

import (
	"github.com/gin-gonic/gin"
	"hris/backend/internal/httputil"
	"hris/backend/internal/middleware"
	"strconv"
)

type Handler struct{ service *Service }

func NewHandler(s *Service) *Handler { return &Handler{service: s} }
func (h *Handler) Users(c *gin.Context) {
	p, l, o := httputil.Pagination(c)
	items, total, err := h.service.Users(c.Request.Context(), p, l, o)
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.List(c, "Users retrieved successfully", items, httputil.NewMeta(p, l, total))
}
func (h *Handler) CreateUser(c *gin.Context) {
	var in CreateUserInput
	if c.ShouldBindJSON(&in) != nil {
		httputil.Error(c, 422, "Invalid user data", "VALIDATION_ERROR")
		return
	}
	item, err := h.service.CreateUser(c.Request.Context(), in, middleware.UserID(c))
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 201, "User created successfully", item)
}

type assignRolesInput struct {
	RoleIDs []uint64 `json:"roleIds" binding:"required"`
}

func (h *Handler) AssignRoles(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httputil.Error(c, 422, "Invalid user ID", "VALIDATION_ERROR")
		return
	}
	var in assignRolesInput
	if c.ShouldBindJSON(&in) != nil {
		httputil.Error(c, 422, "Invalid role assignment", "VALIDATION_ERROR")
		return
	}
	if err := h.service.AssignRoles(c.Request.Context(), id, in.RoleIDs, middleware.UserID(c)); err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 200, "Roles assigned successfully", nil)
}
func (h *Handler) Roles(c *gin.Context) {
	items, err := h.service.Roles(c.Request.Context())
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 200, "Roles retrieved successfully", items)
}
func (h *Handler) Permissions(c *gin.Context) {
	items, err := h.service.Permissions(c.Request.Context())
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 200, "Permissions retrieved successfully", items)
}
func (h *Handler) CreateRole(c *gin.Context) {
	var in RoleInput
	if c.ShouldBindJSON(&in) != nil {
		httputil.Error(c, 422, "Invalid role data", "VALIDATION_ERROR")
		return
	}
	item, err := h.service.CreateRole(c.Request.Context(), in, middleware.UserID(c))
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 201, "Role created successfully", item)
}
