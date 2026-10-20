// Package middleware holds gin middleware.
package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	commonResp "github.com/stanley760/devops/devops-common/response"
	"github.com/stanley760/devops/devops-infra/errs"
)

// Recovery handles panics uniformly: log the stack, then return a 500 error envelope.
// gin.Recovery() is not used because its default output is plain text, which does
// not conform to the unified response format.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic",
					"error", r,
					"method", c.Request.Method,
					"path", c.FullPath(),
					"stack", string(debug.Stack()),
				)
				if c.Writer.Written() {
					// Part of the response has already been written, so it cannot be rewritten
					// as an envelope; only set the 500 status.
					c.AbortWithStatus(http.StatusInternalServerError)
					return
				}
				c.AbortWithStatusJSON(http.StatusInternalServerError, commonResp.BaseResponse[any]{
					Code: errs.CodeInternal,
					Msg:  errs.ErrInternal.Msg,
				})
			}
		}()
		c.Next()
	}
}
