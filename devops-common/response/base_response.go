// Package response defines shared HTTP response contracts.
package response

// BaseResponse is the standard API response envelope.
type BaseResponse[T any] struct {
	Code int    `json:"code" doc:"business code, same as the HTTP status code" required:"true"`
	Msg  string `json:"msg" doc:"response message" required:"true"`
	Data T      `json:"data" doc:"response data" required:"true"`
}
