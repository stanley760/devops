// huma envelope helpers (D4/D7).
//
// huma handler Output.Body reuses these constructors directly; the success shape is
// byte-identical to the Gin version's OK/OKPage; WriteHumaEnvelope writes an envelope
// directly from middleware termination paths.
package response

import (
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/stanley760/devops/devops-common/domain"
	commonResp "github.com/stanley760/devops/devops-common/response"
	"github.com/stanley760/devops/devops-infra/errs"
)

// OKBody is the success response body (for huma handler Output.Body).
func OKBody[T any](data T) commonResp.BaseResponse[T] {
	return commonResp.BaseResponse[T]{Code: errs.CodeOK, Msg: "ok", Data: data}
}

// EmptyBody is the success response body with no data: Data is nil any, serialized
// as "data":null (byte-identical to OK(c, nil)).
func EmptyBody() commonResp.BaseResponse[any] {
	return commonResp.BaseResponse[any]{Code: errs.CodeOK, Msg: "ok", Data: nil}
}

// PageBody is the paginated success response body (byte-identical to OKPage).
func PageBody[T any](page domain.PageResult[T]) commonResp.BaseResponse[domain.PageResult[T]] {
	return commonResp.BaseResponse[domain.PageResult[T]]{
		Code: errs.CodeOK, Msg: "ok",
		Data: page,
	}
}

// WriteHumaEnvelope writes a failure envelope directly to the huma context
// (for middleware 401/403 termination paths).
// Only 4xx/5xx are allowed through, to avoid mistakenly passing a 2xx and creating semantic confusion.
func WriteHumaEnvelope(hctx huma.Context, status int, msg string) {
	if status < 400 || status > 599 {
		status = errs.CodeInternal
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	body, _ := json.Marshal(commonResp.BaseResponse[any]{Code: status, Msg: msg})
	hctx.SetHeader("Content-Type", "application/json; charset=utf-8")
	hctx.SetStatus(status)
	_, _ = hctx.BodyWriter().Write(body)
}
