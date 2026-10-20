package humaerr_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/stanley760/devops/devops-infra/errs"
	"github.com/stanley760/devops/devops-infra/http/humaerr"
)

// smokeInput is the smoke-test input: name has a minLength constraint.
type smokeInput struct {
	Body struct {
		Name string `json:"name" minLength:"2"`
	}
}

// smokeOutput is the smoke-test output.
type smokeOutput struct {
	Body struct {
		OK bool `json:"ok"`
	}
}

// newTestAPI matches production apidoc.New: clear CreateHooks to disable $schema
// injection, guaranteeing byte-level envelope identity (SchemaLinkTransformer would
// add a $schema field and a Link header to responses).
func newTestAPI(t *testing.T, title string) humatest.TestAPI {
	t.Helper()
	cfg := huma.DefaultConfig(title, "v1")
	cfg.CreateHooks = nil
	_, api := humatest.New(t, cfg)
	return api
}

func TestInstallValidation422To400Envelope(t *testing.T) {
	humaerr.Install()
	api := newTestAPI(t, "smoke")

	huma.Register(api, huma.Operation{
		OperationID: "smoke", Method: http.MethodPost, Path: "/smoke",
	}, func(ctx context.Context, in *smokeInput) (*smokeOutput, error) {
		return nil, errs.New(errs.CodeForbidden, "unreachable")
	})

	// minLength violation → huma's internal 422 → Install maps it to a 400 envelope
	resp := api.Post("/smoke", map[string]any{"name": "x"})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("validation failure status = %d, want 400 (422 already mapped), body=%s", resp.Code, resp.Body.String())
	}
	var env humaerr.Envelope
	if err := json.Unmarshal(resp.Body.Bytes(), &env); err != nil {
		t.Fatalf("response is not envelope JSON: %v, body=%s", err, resp.Body.String())
	}
	if env.Code != 400 {
		t.Fatalf("envelope code = %d, want 400", env.Code)
	}
	if !strings.Contains(env.Msg, "parameter validation failed") {
		t.Fatalf("envelope msg is missing the prefix: %q", env.Msg)
	}
	if env.Data != nil {
		t.Fatalf("envelope data should be null: %v", env.Data)
	}
}

func TestInstallBizErrorEnvelope(t *testing.T) {
	humaerr.Install()
	api := newTestAPI(t, "smoke2")

	huma.Register(api, huma.Operation{
		OperationID: "biz", Method: http.MethodPost, Path: "/biz",
	}, func(ctx context.Context, _ *smokeInput) (*smokeOutput, error) {
		return nil, errs.ToHumaError(errs.New(errs.CodeForbidden, "operation not permitted"))
	})

	// valid body + handler returns a BizError → 403 envelope (via errs.ToHumaError → NewError)
	resp := api.Post("/biz", map[string]any{"name": "ab"})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("BizError status = %d, want 403, body=%s", resp.Code, resp.Body.String())
	}
	var env humaerr.Envelope
	if err := json.Unmarshal(resp.Body.Bytes(), &env); err != nil {
		t.Fatalf("response is not envelope JSON: %v", err)
	}
	if env.Code != 403 || env.Msg != "operation not permitted" {
		t.Fatalf("envelope mismatch: %+v", env)
	}
}

func TestInstallContentTypeJSON(t *testing.T) {
	humaerr.Install()
	api := newTestAPI(t, "smoke3")

	huma.Register(api, huma.Operation{
		OperationID: "ct", Method: http.MethodPost, Path: "/ct",
	}, func(ctx context.Context, _ *smokeInput) (*smokeOutput, error) {
		return nil, errs.ToHumaError(errs.New(errs.CodeInternal, "internal error"))
	})

	resp := api.Post("/ct", map[string]any{"name": "ab"})
	ct := resp.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json (without ContentTypeFilter it must not negotiate to problem+json)", ct)
	}
	// contract protection: the response must not have a $schema field injected by SchemaLinkTransformer
	if strings.Contains(resp.Body.String(), "$schema") {
		t.Fatalf("the response was injected with $schema, breaking the byte-level envelope contract: %s", resp.Body.String())
	}
}
