// Package response provides a unified HTTP response format and error output.
//
// All business endpoints return a BaseResponse envelope:
//
//	{"code": 200, "msg": "ok", "data": {...}}
//
// code is identical to the HTTP status code. Business handlers report failures by
// returning an error; after being adapted by Wrap, FailErr converts them uniformly,
// so every call site avoids hand-writing error responses:
//
//	r.GET("/api/v1/users", response.Wrap(h.ListUsers))
//
// Inside a handler, the success path calls OK / OKPage directly to write the response.
package response

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stanley760/devops/devops-common/domain"
	commonResp "github.com/stanley760/devops/devops-common/response"
	"github.com/stanley760/devops/devops-infra/errs"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type BaseResponse[T any] = commonResp.BaseResponse[T]

// Handler is the business handler signature: report failures by returning an error,
// and call OK / OKPage directly on success to write the response.
type Handler func(c *gin.Context) error

// OK is the success response. Passing nil for data means no data (delete, update, etc.).
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, BaseResponse[any]{
		Code: errs.CodeOK,
		Msg:  "ok",
		Data: data,
	})
}

// OKPage is the paginated success response.
func OKPage[T any](c *gin.Context, page domain.PageResult[T]) {
	c.JSON(http.StatusOK, BaseResponse[domain.PageResult[T]]{
		Code: errs.CodeOK,
		Msg:  "ok",
		Data: page,
	})
}

// Fail is the failure response; code also serves as the HTTP status code.
// Only 4xx/5xx are allowed through, to avoid mistakenly passing a 2xx and creating semantic confusion.
func Fail(c *gin.Context, code int, msg string) {
	if code < 400 || code > 599 {
		code = errs.CodeInternal
	}
	if msg == "" {
		msg = http.StatusText(code)
	}
	c.AbortWithStatusJSON(code, BaseResponse[any]{Code: code, Msg: msg})
}

// FailErr is the unified error output: a *errs.BizError is recognized and its
// code/msg used, while other unknown errors are logged and treated as 500, without
// leaking internal details to the client.
// A BizError is an expected business failure (e.g. a bad parameter), so it is not logged.
func FailErr(c *gin.Context, err error) {
	var be *errs.BizError
	if errors.As(err, &be) {
		Fail(c, be.Code, be.Msg)
		return
	}

	// Handlers habitually return the error from c.ShouldBindJSON directly;
	// binding/validation errors are client problems, so classify them as 400 rather than a 500 fallback.
	if msg, ok := asParamError(err); ok {
		Fail(c, errs.CodeParam, msg)
		return
	}

	slog.Error("unclassified error", "err", err, "method", c.Request.Method, "path", c.FullPath())
	Fail(c, errs.CodeInternal, errs.ErrInternal.Msg)
}

// asParamError recognizes request binding/validation errors and returns a user-facing message.
func asParamError(err error) (string, bool) {
	var valErrs validator.ValidationErrors
	if errors.As(err, &valErrs) {
		var b strings.Builder
		for i, fe := range valErrs {
			if i > 0 {
				b.WriteString("; ")
			}
			fmt.Fprintf(&b, "field %s failed %s validation", fe.Field(), fe.Tag())
		}
		return "parameter validation failed: " + b.String(), true
	}
	var synErr *json.SyntaxError
	if errors.As(err, &synErr) {
		return "request body is not valid JSON", true
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return "request body field type error", true
	}
	var parseErr *time.ParseError
	if errors.As(err, &parseErr) {
		return "request body time format error", true
	}
	if errors.Is(err, io.EOF) {
		return "request body must not be empty", true
	}
	return "", false
}

// Wrap adapts a Handler into a gin.HandlerFunc and handles errors uniformly:
//   - returns a BizError (including one wrapped by fmt.Errorf("%w")) → respond with its code/msg
//   - returns a binding/validation error (returned directly from ShouldBindJSON) → 400 with a field-level message
//   - returns any other error → 500, with details only going to the log
//   - returns nil without writing a response (a missed-write bug) → fall back to 500 to avoid hanging the request
//   - writes a response and then returns an error → output cannot be overwritten, so log it and abort
func Wrap(h Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		err := h(c)
		switch {
		case err == nil:
			if !c.Writer.Written() {
				slog.Error("handler did not write a response", "method", c.Request.Method, "path", c.FullPath())
				Fail(c, errs.CodeInternal, errs.ErrInternal.Msg)
			}
		case c.Writer.Written():
			slog.Error("handler already wrote a response but still returned an error", "err", err, "method", c.Request.Method, "path", c.FullPath())
			c.Abort()
		default:
			FailErr(c, err)
		}
	}
}
