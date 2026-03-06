package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorResponse is the standard error format.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// NotFound returns a 404 error response.
func NotFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, ErrorResponse{Error: msg, Code: "NOT_FOUND"})
}

// BadRequest returns a 400 error response.
func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, ErrorResponse{Error: msg, Code: "BAD_REQUEST"})
}

// InternalError returns a 500 error response.
func InternalError(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, ErrorResponse{Error: msg, Code: "INTERNAL_ERROR"})
}
