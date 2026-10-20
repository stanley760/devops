// Package humaerr aligns huma's error output with the unified envelope (D1/D6/D7).
//
// huma's default error response is RFC7807 problem+json, which does not match the
// project's BaseResponse envelope contract.
// After Install() overrides the package-level variable huma.NewError (must be called
// before any huma request handling):
//   - a handler returns errs.ToHumaError(err) → goes through NewError → envelope ✔
//   - huma schema validation failure (422) → mapped to 400, validation details merged into msg ✔
//   - a custom Envelope without ContentTypeFilter keeps Content-Type as application/json ✔
//
// WriteErr obtains the error object via NewErrorWithContext (which delegates to
// NewError by default) and re-reads the status code with GetStatus() — the status
// mapping inside Install therefore applies globally to the validation path as well.
package humaerr

import (
	"github.com/danielgtaylor/huma/v2"
	"net/http"
	"strings"
)

// Envelope is the error envelope (byte-identical to the failure shape of
// common/response.BaseResponse[any]).
type Envelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"` // always nil; serialized as "data":null, matching the Gin version
}

// GetStatus implements huma.StatusError.
func (e *Envelope) GetStatus() int { return e.Code }

// Error implements error / huma.StatusError.
func (e *Envelope) Error() string { return e.Msg }

// Install overrides huma.NewError (package-level variable), effective globally;
// call it during main startup.
// 422 (huma schema validation failure) is mapped to 400, with validation details
// merged into msg, matching the legacy Gin contract.
func Install() {
	huma.NewError = func(status int, msg string, detailErrs ...error) huma.StatusError {
		if status == http.StatusUnprocessableEntity {
			status = http.StatusBadRequest
			msg = formatValidation(msg, detailErrs)
		}
		if msg == "" {
			msg = http.StatusText(status)
		}
		return &Envelope{Code: status, Msg: msg}
	}
}

// formatValidation aggregates validation details into a user-facing message.
func formatValidation(msg string, errs []error) string {
	if len(errs) == 0 {
		return "parameter validation failed: " + msg
	}
	var b strings.Builder
	b.WriteString("parameter validation failed: ")
	for i, err := range errs {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(err.Error())
	}
	return b.String()
}

// envelopeSchema is the error envelope schema (docs form matches the runtime output).
func envelopeSchema() *huma.Schema {
	return &huma.Schema{
		Type: "object",
		Properties: map[string]*huma.Schema{
			"code": {Type: "integer", Description: "business code, identical to the HTTP status code"},
			"msg":  {Type: "string", Description: "message"},
			"data": {Description: "business data; always null on failure"},
		},
	}
}

// ErrResponses generates envelope response declarations for the given status codes (D6).
// Paired with Operation.Responses, it replaces huma's default problem+json error docs.
func ErrResponses(codes map[string]string) map[string]*huma.Response {
	content := map[string]*huma.MediaType{
		"application/json": {Schema: envelopeSchema()},
	}
	responses := make(map[string]*huma.Response, len(codes))
	for code, desc := range codes {
		responses[code] = &huma.Response{Description: desc, Content: content}
	}
	return responses
}

// CommonErrResponses provides envelope response declarations for the common
// error codes (400/401/403/404/500).
func CommonErrResponses() map[string]*huma.Response {
	return ErrResponses(map[string]string{
		"400": "bad request",
		"401": "not logged in or session expired",
		"403": "operation not permitted",
		"404": "resource not found",
		"500": "internal server error",
	})
}
