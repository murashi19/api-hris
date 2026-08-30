package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"hris/backend/internal/config"
	"hris/backend/internal/httputil"
	"hris/backend/internal/middleware"
)

type Handler struct {
	service *Service
	cfg     config.Config
}

func NewHandler(service *Service, cfg config.Config) *Handler {
	return &Handler{service: service, cfg: cfg}
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=200"`
}

func (h *Handler) Login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httputil.Error(c, http.StatusUnprocessableEntity, "Email and password are required", "VALIDATION_ERROR")
		return
	}
	result, err := h.service.Login(c.Request.Context(), request.Email, request.Password)
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	h.setRefreshCookie(c, result.RefreshToken)
	httputil.OK(c, http.StatusOK, "Login successful", result)
}

func (h *Handler) Refresh(c *gin.Context) {
	raw, _ := c.Cookie(h.cfg.RefreshCookieName)
	result, err := h.service.Refresh(c.Request.Context(), raw)
	if err != nil {
		h.clearRefreshCookie(c)
		httputil.WriteDomainError(c, err)
		return
	}
	h.setRefreshCookie(c, result.RefreshToken)
	httputil.OK(c, http.StatusOK, "Access token refreshed", result)
}

func (h *Handler) Logout(c *gin.Context) {
	raw, _ := c.Cookie(h.cfg.RefreshCookieName)
	if err := h.service.Logout(c.Request.Context(), raw); err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	h.clearRefreshCookie(c)
	httputil.OK(c, http.StatusOK, "Logout successful", nil)
}

func (h *Handler) Me(c *gin.Context) {
	user, err := h.service.Me(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		httputil.WriteDomainError(c, err)
		return
	}
	httputil.OK(c, http.StatusOK, "Current user retrieved successfully", user)
}

func (h *Handler) setRefreshCookie(c *gin.Context, token string) {
	maxAge := int(h.cfg.RefreshTokenTTL / time.Second)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(h.cfg.RefreshCookieName, token, maxAge, "/api/auth", "", h.cfg.RefreshCookieSecure, true)
}

func (h *Handler) clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(h.cfg.RefreshCookieName, "", -1, "/api/auth", "", h.cfg.RefreshCookieSecure, true)
}
