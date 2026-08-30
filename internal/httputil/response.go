package httputil

import "github.com/gin-gonic/gin"

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Meta    any    `json:"meta,omitempty"`
	Code    string `json:"code,omitempty"`
	Errors  any    `json:"errors,omitempty"`
}

type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

func OK(c *gin.Context, status int, message string, data any) {
	c.JSON(status, Response{Success: true, Message: message, Data: data})
}

func List(c *gin.Context, message string, data any, meta Meta) {
	c.JSON(200, Response{Success: true, Message: message, Data: data, Meta: meta})
}

func Error(c *gin.Context, status int, message, code string) {
	c.AbortWithStatusJSON(status, Response{Success: false, Message: message, Code: code})
}
