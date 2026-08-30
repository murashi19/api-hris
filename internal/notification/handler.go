package notification

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"hris/backend/internal/httputil"
	"hris/backend/internal/middleware"
	"hris/backend/internal/model"
)

type Handler struct{ db *gorm.DB }

func NewHandler(db *gorm.DB) *Handler { return &Handler{db: db} }
func (h *Handler) List(c *gin.Context) {
	page, limit, offset := httputil.Pagination(c)
	q := h.db.WithContext(c.Request.Context()).Model(&model.Notification{}).Where("user_id=?", middleware.UserID(c))
	if c.Query("unread") == "true" {
		q = q.Where("read_at IS NULL")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	var items []model.Notification
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.List(c, "Notifications retrieved successfully", items, httputil.NewMeta(page, limit, total))
}
func (h *Handler) Read(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httputil.Error(c, 422, "Invalid notification ID", "VALIDATION_ERROR")
		return
	}
	now := time.Now().UTC()
	result := h.db.WithContext(c.Request.Context()).Model(&model.Notification{}).Where("id=? AND user_id=?", id, middleware.UserID(c)).Update("read_at", now)
	if result.Error != nil {
		httputil.WriteDomainError(c, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		httputil.Error(c, 404, "Notification not found", "NOTIFICATION_NOT_FOUND")
		return
	}
	httputil.OK(c, http.StatusOK, "Notification marked as read", map[string]any{"id": id, "readAt": now})
}
