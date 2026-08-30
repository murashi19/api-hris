package httputil

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func Pagination(c *gin.Context) (page, limit, offset int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit, (page - 1) * limit
}

func NewMeta(page, limit int, total int64) Meta {
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}
	return Meta{Page: page, Limit: limit, Total: total, TotalPages: totalPages}
}
