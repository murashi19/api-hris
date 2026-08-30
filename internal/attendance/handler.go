package attendance

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"hris/backend/internal/httputil"
	"hris/backend/internal/middleware"
)

type Handler struct{ service *Service }

func NewHandler(s *Service) *Handler { return &Handler{service: s} }

type clockInRequest struct {
	Notes string `json:"notes" binding:"max=500"`
}

func (h *Handler) ClockIn(c *gin.Context) {
	employeeID, ok := middleware.EmployeeID(c)
	if !ok {
		httputil.Error(c, 422, "User is not linked to an employee", "EMPLOYEE_LINK_REQUIRED")
		return
	}
	var in clockInRequest
	if err := c.ShouldBindJSON(&in); err != nil && !errors.Is(err, io.EOF) {
		httputil.Error(c, 422, "Invalid attendance data", "VALIDATION_ERROR")
		return
	}
	item, err := h.service.ClockIn(c.Request.Context(), employeeID, middleware.UserID(c), in.Notes)
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, http.StatusCreated, "Clock-in successful", item)
}
func (h *Handler) ClockOut(c *gin.Context) {
	employeeID, ok := middleware.EmployeeID(c)
	if !ok {
		httputil.Error(c, 422, "User is not linked to an employee", "EMPLOYEE_LINK_REQUIRED")
		return
	}
	item, err := h.service.ClockOut(c.Request.Context(), employeeID, middleware.UserID(c))
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 200, "Clock-out successful", item)
}
func (h *Handler) Mine(c *gin.Context) {
	employeeID, ok := middleware.EmployeeID(c)
	if !ok {
		httputil.Error(c, 422, "User is not linked to an employee", "EMPLOYEE_LINK_REQUIRED")
		return
	}
	page, limit, offset := httputil.Pagination(c)
	items, total, err := h.service.ListMine(c.Request.Context(), employeeID, page, limit, offset)
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.List(c, "Attendance retrieved successfully", items, httputil.NewMeta(page, limit, total))
}

func (h *Handler) Team(c *gin.Context) {
	managerID, ok := middleware.EmployeeID(c)
	if !ok {
		httputil.Error(c, 422, "User is not linked to an employee", "EMPLOYEE_LINK_REQUIRED")
		return
	}
	page, limit, offset := httputil.Pagination(c)
	start, ok := parseDate(c, "startDate")
	if !ok {
		return
	}
	end, ok := parseDate(c, "endDate")
	if !ok {
		return
	}
	items, total, err := h.service.ListTeam(c.Request.Context(), managerID, start, end, page, limit, offset)
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.List(c, "Team attendance retrieved successfully", items, httputil.NewMeta(page, limit, total))
}
func (h *Handler) All(c *gin.Context) {
	page, limit, offset := httputil.Pagination(c)
	var emp *uint64
	if raw := c.Query("employeeId"); raw != "" {
		v, e := strconv.ParseUint(raw, 10, 64)
		if e != nil {
			httputil.Error(c, 422, "Invalid employee ID", "VALIDATION_ERROR")
			return
		}
		emp = &v
	}
	start, ok := parseDate(c, "startDate")
	if !ok {
		return
	}
	end, ok := parseDate(c, "endDate")
	if !ok {
		return
	}
	items, total, err := h.service.ListAll(c.Request.Context(), emp, start, end, page, limit, offset)
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.List(c, "Attendance retrieved successfully", items, httputil.NewMeta(page, limit, total))
}
func parseDate(c *gin.Context, key string) (*time.Time, bool) {
	raw := c.Query(key)
	if raw == "" {
		return nil, true
	}
	v, err := time.Parse("2006-01-02", raw)
	if err != nil {
		httputil.Error(c, 422, "Invalid date filter", "VALIDATION_ERROR")
		return nil, false
	}
	return &v, true
}
