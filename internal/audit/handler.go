package audit

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"hris/backend/internal/httputil"
	"hris/backend/internal/model"
)

type Handler struct{ db *gorm.DB }

func NewHandler(db *gorm.DB) *Handler { return &Handler{db: db} }
func (h *Handler) List(c *gin.Context) {
	page, limit, offset := httputil.Pagination(c)
	q := h.db.WithContext(c.Request.Context()).Model(&model.AuditLog{})
	if action := c.Query("action"); action != "" {
		q = q.Where("action=?", action)
	}
	if entityType := c.Query("entityType"); entityType != "" {
		q = q.Where("entity_type=?", entityType)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	var items []model.AuditLog
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.List(c, "Audit logs retrieved successfully", items, httputil.NewMeta(page, limit, total))
}
