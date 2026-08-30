package httputil

import "errors"

type DomainError struct {
	Code    string
	Message string
	Status  int
}

func (e *DomainError) Error() string { return e.Message }

func NewDomainError(status int, code, message string) error {
	return &DomainError{Status: status, Code: code, Message: message}
}

func WriteDomainError(c interface{ AbortWithStatusJSON(int, any) }, err error) {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		c.AbortWithStatusJSON(domainErr.Status, Response{Success: false, Message: domainErr.Message, Code: domainErr.Code})
		return
	}
	c.AbortWithStatusJSON(500, Response{Success: false, Message: "Internal server error", Code: "INTERNAL_ERROR"})
}
