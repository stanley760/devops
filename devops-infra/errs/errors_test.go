package errs_test

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/stanley760/devops/devops-infra/errs"
)

func TestToHumaErrorMapsBizStatusAndMessage(t *testing.T) {
	got := errs.ToHumaError(errs.ErrForbidden)

	var statusErr huma.StatusError
	if !errors.As(got, &statusErr) {
		t.Fatalf("ToHumaError() = %T, want huma.StatusError", got)
	}
	if statusErr.GetStatus() != errs.CodeForbidden {
		t.Fatalf("status = %d, want %d", statusErr.GetStatus(), errs.CodeForbidden)
	}
	if got.Error() != errs.ErrForbidden.Msg {
		t.Fatalf("message = %q, want %q", got.Error(), errs.ErrForbidden.Msg)
	}
}

func TestToHumaErrorNormalizesInvalidStatus(t *testing.T) {
	got := errs.ToHumaError(errs.New(200, "invalid success response"))

	var statusErr huma.StatusError
	if !errors.As(got, &statusErr) {
		t.Fatalf("ToHumaError() = %T, want huma.StatusError", got)
	}
	if statusErr.GetStatus() != errs.CodeInternal {
		t.Fatalf("status = %d, want %d", statusErr.GetStatus(), errs.CodeInternal)
	}
}

func TestToHumaErrorHidesUnknownErrorDetail(t *testing.T) {
	cause := errors.New("database password rejected")
	got := errs.ToHumaError(cause)

	var statusErr huma.StatusError
	if !errors.As(got, &statusErr) {
		t.Fatalf("ToHumaError() = %T, want huma.StatusError", got)
	}
	if statusErr.GetStatus() != errs.CodeInternal {
		t.Fatalf("status = %d, want %d", statusErr.GetStatus(), errs.CodeInternal)
	}
	if strings.Contains(got.Error(), cause.Error()) {
		t.Fatalf("unknown error must not leak detail: %q", got.Error())
	}
}

func TestToHumaErrorNil(t *testing.T) {
	if got := errs.ToHumaError(nil); got != nil {
		t.Fatalf("ToHumaError(nil) = %v, want nil", got)
	}
}

// captureSlog redirects the default slog output to a buffer and returns a restore function.
func captureSlog(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	var buf bytes.Buffer
	origin := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	return &buf, func() { slog.SetDefault(origin) }
}

func TestWrapCapturesCallerStack(t *testing.T) {
	cause := errors.New("dial tcp: connection refused")
	got := errs.Wrap(cause, errs.CodeInternal, "ID generation failed")

	stack := got.Stack()
	if !strings.Contains(stack, "TestWrapCapturesCallerStack") {
		t.Fatalf("Stack() did not land on the business call point:\n%s", stack)
	}
	if strings.Contains(stack, "devops-infra/errs.") {
		t.Fatalf("Stack() must not contain frames from the errs package itself:\n%s", stack)
	}
	if !strings.Contains(got.Error(), cause.Error()) {
		t.Fatalf("Error() must keep the underlying cause: %q", got.Error())
	}
}

func TestStackOfZeroValueBizErrorIsEmpty(t *testing.T) {
	if got := (&errs.BizError{}).Stack(); got != "" {
		t.Fatalf("zero-value BizError Stack() = %q, want empty string", got)
	}
}

func TestToHumaErrorLogsUnclassifiedWithStack(t *testing.T) {
	buf, restore := captureSlog(t)
	defer restore()

	cause := errors.New("sql: connection refused")
	got := errs.ToHumaError(cause)

	var statusErr huma.StatusError
	if !errors.As(got, &statusErr) || statusErr.GetStatus() != errs.CodeInternal {
		t.Fatalf("ToHumaError(non-BizError) should be 500, got %v", got)
	}
	if log := buf.String(); !strings.Contains(log, "unclassified error") ||
		!strings.Contains(log, cause.Error()) ||
		!strings.Contains(log, "TestToHumaErrorLogsUnclassifiedWithStack") {
		t.Fatalf("unclassified error log must contain the error chain and the call-point stack, got:\n%s", log)
	}
}

func TestToHumaErrorLogs5xxBizErrorWithCauseAndStack(t *testing.T) {
	buf, restore := captureSlog(t)
	defer restore()

	cause := errors.New("Error 1062: Duplicate entry")
	_ = errs.ToHumaError(errs.Wrap(cause, errs.CodeInternal, "failed to create user"))

	log := buf.String()
	if !strings.Contains(log, "business error (5xx)") ||
		!strings.Contains(log, cause.Error()) ||
		!strings.Contains(log, "TestToHumaErrorLogs5xxBizErrorWithCauseAndStack") {
		t.Fatalf("5xx log must contain the business semantics, underlying cause, and stack, got:\n%s", log)
	}
}

func TestToHumaErrorSkips4xxLogging(t *testing.T) {
	buf, restore := captureSlog(t)
	defer restore()

	_ = errs.ToHumaError(errs.New(errs.CodeParam, "username already exists"))

	if log := buf.String(); log != "" {
		t.Fatalf("4xx are expected business rejections and must not be logged as errors, got:\n%s", log)
	}
}
