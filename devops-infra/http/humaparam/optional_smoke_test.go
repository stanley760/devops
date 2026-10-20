package humaparam_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"net/http/httptest"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/stanley760/devops/devops-infra/http/humaerr"
	"github.com/stanley760/devops/devops-infra/http/humaparam"
)

// optInput is the input with an Optional query parameter (reproducing the *int8
// semantics of pagination filtering).
type optInput struct {
	Status humaparam.Optional[int8] `query:"status" minimum:"0" maximum:"1"`
}

type optOutput struct {
	Body struct {
		Status *int8 `json:"status"`
	}
}

func newOptAPI(t *testing.T) humatest.TestAPI {
	cfg := huma.DefaultConfig("p", "v1")
	cfg.CreateHooks = nil // aligned with production apidoc.New: disable $schema injection
	_, api := humatest.New(t, cfg)
	registerOpt(api)
	return api
}

func registerOpt(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "opt", Method: http.MethodGet, Path: "/opt",
	}, func(ctx context.Context, in *optInput) (*optOutput, error) {
		out := &optOutput{}
		out.Body.Status = in.Status.Ptr()
		return out, nil
	})
}

func optStatus(t *testing.T, resp *httptest.ResponseRecorder) *int8 {
	t.Helper()
	var out struct {
		Status *int8 `json:"status"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("response JSON: %v", err)
	}
	return out.Status
}

func TestOptionalAbsent(t *testing.T) {
	api := newOptAPI(t)

	resp := api.Get("/opt")
	if resp.Code != http.StatusOK {
		t.Fatalf("default request status = %d, want 200, body=%s", resp.Code, resp.Body.String())
	}
	if s := optStatus(t, resp); s != nil {
		t.Fatalf("default status should be nil, got %v", *s)
	}
}

func TestOptionalExplicitZero(t *testing.T) {
	api := newOptAPI(t)

	// explicit 0 (a valid business value) → IsSet=true, Ptr() points at 0
	resp := api.Get("/opt?status=0")
	if resp.Code != http.StatusOK {
		t.Fatalf("explicit 0 status = %d, want 200, body=%s", resp.Code, resp.Body.String())
	}
	s := optStatus(t, resp)
	if s == nil || *s != 0 {
		t.Fatalf("explicit 0 should be IsSet with value 0, got %v", s)
	}
}

func TestOptionalExplicitOne(t *testing.T) {
	api := newOptAPI(t)

	resp := api.Get("/opt?status=1")
	if resp.Code != http.StatusOK {
		t.Fatalf("explicit 1 status = %d, want 200, body=%s", resp.Code, resp.Body.String())
	}
	s := optStatus(t, resp)
	if s == nil || *s != 1 {
		t.Fatalf("explicit 1 should be IsSet with value 1, got %v", s)
	}
}

func TestOptionalConstraintRejected(t *testing.T) {
	humaerr.Install()
	api := newOptAPI(t)

	// explicit 2 violates maximum:1 → 422→400 envelope
	resp := api.Get("/opt?status=2")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("out-of-range status = %d, want 400, body=%s", resp.Code, resp.Body.String())
	}
}
