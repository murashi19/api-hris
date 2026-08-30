package department

import (
	"github.com/gin-gonic/gin"
	"hris/backend/internal/httputil"
	"hris/backend/internal/middleware"
	"net/http"
	"strconv"
)

type Handler struct{ service *Service }

func NewHandler(s *Service) *Handler { return &Handler{service: s} }
func (h *Handler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context())
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, http.StatusOK, "Departments retrieved successfully", items)
}
func (h *Handler) Create(c *gin.Context) {
	var in Input
	if c.ShouldBindJSON(&in) != nil {
		httputil.Error(c, 422, "Invalid department data", "VALIDATION_ERROR")
		return
	}
	item, err := h.service.Create(c.Request.Context(), in, middleware.UserID(c))
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 201, "Department created successfully", item)
}
func (h *Handler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httputil.Error(c, 422, "Invalid department ID", "VALIDATION_ERROR")
		return
	}
	var in Input
	if c.ShouldBindJSON(&in) != nil {
		httputil.Error(c, 422, "Invalid department data", "VALIDATION_ERROR")
		return
	}
	item, err := h.service.Update(c.Request.Context(), id, in, middleware.UserID(c))
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 200, "Department updated successfully", item)
}
