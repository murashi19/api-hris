package leave

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"hris/backend/internal/httputil"
	"hris/backend/internal/middleware"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Types(c *gin.Context) {
	items, err := h.service.Types(c.Request.Context())
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 200, "Leave types retrieved successfully", items)
}
func (h *Handler) CreateType(c *gin.Context) {
	var in LeaveTypeInput
	if c.ShouldBindJSON(&in) != nil {
		httputil.Error(c, 422, "Invalid leave type data", "VALIDATION_ERROR")
		return
	}
	item, err := h.service.CreateType(c.Request.Context(), in, middleware.UserID(c))
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 201, "Leave type created successfully", item)
}
func (h *Handler) UpdateType(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httputil.Error(c, 422, "Invalid leave type ID", "VALIDATION_ERROR")
		return
	}
	var in LeaveTypeInput
	if c.ShouldBindJSON(&in) != nil {
		httputil.Error(c, 422, "Invalid leave type data", "VALIDATION_ERROR")
		return
	}
	item, err := h.service.UpdateType(c.Request.Context(), id, in, middleware.UserID(c))
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 200, "Leave type updated successfully", item)
}
func (h *Handler) SetEntitlement(c *gin.Context) {
	var in BalanceInput
	if c.ShouldBindJSON(&in) != nil {
		httputil.Error(c, 422, "Invalid entitlement data", "VALIDATION_ERROR")
		return
	}
	item, err := h.service.SetEntitlement(c.Request.Context(), in, middleware.UserID(c))
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 200, "Leave entitlement set successfully", item)
}
func (h *Handler) AdjustBalance(c *gin.Context) {
	var in AdjustmentInput
	if c.ShouldBindJSON(&in) != nil {
		httputil.Error(c, 422, "Invalid balance adjustment", "VALIDATION_ERROR")
		return
	}
	item, err := h.service.AdjustBalance(c.Request.Context(), in, middleware.UserID(c))
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 200, "Leave balance adjusted successfully", item)
}
func (h *Handler) Balances(c *gin.Context) {
	employeeID, ok := employeeRequired(c)
	if !ok {
		return
	}
	items, err := h.service.Balances(c.Request.Context(), employeeID)
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 200, "Leave balances retrieved successfully", items)
}
func (h *Handler) Submit(c *gin.Context) {
	employeeID, ok := employeeRequired(c)
	if !ok {
		return
	}
	var in SubmitInput
	if c.ShouldBindJSON(&in) != nil {
		httputil.Error(c, 422, "Invalid leave request data", "VALIDATION_ERROR")
		return
	}
	item, err := h.service.Submit(c.Request.Context(), employeeID, middleware.UserID(c), in)
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, http.StatusCreated, "Leave request submitted successfully", item)
}
func (h *Handler) Mine(c *gin.Context) {
	employeeID, ok := employeeRequired(c)
	if !ok {
		return
	}
	page, limit, offset := httputil.Pagination(c)
	items, total, err := h.service.Mine(c.Request.Context(), employeeID, page, limit, offset)
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.List(c, "Leave requests retrieved successfully", items, httputil.NewMeta(page, limit, total))
}
func (h *Handler) Get(c *gin.Context) {
	id, ok := leaveID(c)
	if !ok {
		return
	}
	employeeID, _ := middleware.EmployeeID(c)
	canReadAll := middleware.HasPermission(c, "leave.all.read")
	item, err := h.service.Get(c.Request.Context(), id, middleware.UserID(c), employeeID, canReadAll)
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 200, "Leave request retrieved successfully", item)
}
func (h *Handler) Approvals(c *gin.Context) {
	employeeID, _ := middleware.EmployeeID(c)
	isHR := middleware.HasPermission(c, "leave.hr.approve")
	if !isHR && employeeID == 0 {
		httputil.Error(c, 422, "User is not linked to an employee", "EMPLOYEE_LINK_REQUIRED")
		return
	}
	page, limit, offset := httputil.Pagination(c)
	items, total, err := h.service.ApprovalQueue(c.Request.Context(), employeeID, isHR, page, limit, offset)
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.List(c, "Leave approvals retrieved successfully", items, httputil.NewMeta(page, limit, total))
}
func (h *Handler) ManagerApprove(c *gin.Context) {
	id, ok := leaveID(c)
	if !ok {
		return
	}
	employeeID, ok := employeeRequired(c)
	if !ok {
		return
	}
	item, err := h.service.ManagerApprove(c.Request.Context(), id, middleware.UserID(c), employeeID)
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 200, "Leave request approved by manager", item)
}
func (h *Handler) HRApprove(c *gin.Context) {
	id, ok := leaveID(c)
	if !ok {
		return
	}
	item, err := h.service.HRApprove(c.Request.Context(), id, middleware.UserID(c))
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 200, "Leave request approved by HR", item)
}

type rejectInput struct {
	Reason string `json:"reason" binding:"required,max=1000"`
}

func (h *Handler) Reject(c *gin.Context) {
	id, ok := leaveID(c)
	if !ok {
		return
	}
	var in rejectInput
	if c.ShouldBindJSON(&in) != nil {
		httputil.Error(c, 422, "Rejection reason is required", "REJECTION_REASON_REQUIRED")
		return
	}
	employeeID, _ := middleware.EmployeeID(c)
	item, err := h.service.Reject(c.Request.Context(), id, middleware.UserID(c), employeeID, middleware.HasPermission(c, "leave.hr.approve"), in.Reason)
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 200, "Leave request rejected", item)
}
func (h *Handler) Cancel(c *gin.Context) {
	id, ok := leaveID(c)
	if !ok {
		return
	}
	employeeID, ok := employeeRequired(c)
	if !ok {
		return
	}
	item, err := h.service.Cancel(c.Request.Context(), id, middleware.UserID(c), employeeID)
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, 200, "Leave request cancelled", item)
}
func employeeRequired(c *gin.Context) (uint64, bool) {
	id, ok := middleware.EmployeeID(c)
	if !ok {
		httputil.Error(c, 422, "User is not linked to an employee", "EMPLOYEE_LINK_REQUIRED")
		return 0, false
	}
	return id, true
}
func leaveID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		httputil.Error(c, 422, "Invalid leave request ID", "VALIDATION_ERROR")
		return 0, false
	}
	return id, true
}
