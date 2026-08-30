package employee

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"hris/backend/internal/httputil"
	"hris/backend/internal/middleware"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) List(c *gin.Context) {
	page, limit, offset := httputil.Pagination(c)
	var departmentID *uint64
	if raw := c.Query("departmentId"); raw != "" {
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			httputil.Error(c, 422, "Invalid department ID", "VALIDATION_ERROR")
			return
		}
		departmentID = &value
	}
	items, total, err := h.service.List(c.Request.Context(), c.Query("search"), c.Query("status"), departmentID, page, limit, offset)
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.List(c, "Employees retrieved successfully", items, httputil.NewMeta(page, limit, total))
}

func (h *Handler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, http.StatusOK, "Employee retrieved successfully", item)
}

func (h *Handler) Create(c *gin.Context) {
	var input Input
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.Error(c, 422, "Invalid employee data", "VALIDATION_ERROR")
		return
	}
	item, err := h.service.Create(c.Request.Context(), input, middleware.UserID(c))
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, http.StatusCreated, "Employee created successfully", item)
}

func (h *Handler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var input Input
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.Error(c, 422, "Invalid employee data", "VALIDATION_ERROR")
		return
	}
	item, err := h.service.Update(c.Request.Context(), id, input, middleware.UserID(c))
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, http.StatusOK, "Employee updated successfully", item)
}

func parseID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		httputil.Error(c, 422, "Invalid employee ID", "VALIDATION_ERROR")
		return 0, false
	}
	return id, true
}
