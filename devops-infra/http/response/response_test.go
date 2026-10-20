package response_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/stanley760/devops/devops-common/domain"
	"github.com/stanley760/devops/devops-infra/errs"
	"github.com/stanley760/devops/devops-infra/http/response"
)

// run executes the handler through a real gin engine and returns its response.
func run(t *testing.T, h gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", h)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	return w
}

// env loosely parses the unified envelope.
type env struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func TestOK(t *testing.T) {
	w := run(t, func(c *gin.Context) {
		response.OK(c, gin.H{"id": 1})
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var e env
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("response is not a unified envelope: %v", err)
	}
	if e.Code != errs.CodeOK || e.Msg != "ok" {
		t.Fatalf("code=%d msg=%q, want code=200 msg=ok", e.Code, e.Msg)
	}
	if !strings.Contains(w.Body.String(), `"id":1`) {
		t.Fatalf("data missing: %s", w.Body.String())
	}
}

func TestOKPage(t *testing.T) {
	w := run(t, func(c *gin.Context) {
		response.OKPage(c, domain.PageResult[int]{
			List:     []int{1, 2},
			Total:    21,
			PageNum:  2,
			PageSize: 10,
			Pages:    3,
		})
	})

	var e struct {
		Code int `json:"code"`
		Data struct {
			List     []int `json:"list"`
			Total    int64 `json:"total"`
			PageNum  int   `json:"page_num"`
			PageSize int   `json:"page_size"`
			Pages    int64 `json:"pages"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("response is not a unified envelope: %v", err)
	}
	if e.Code != errs.CodeOK || len(e.Data.List) != 2 || e.Data.Total != 21 ||
		e.Data.PageNum != 2 || e.Data.PageSize != 10 || e.Data.Pages != 3 {
		t.Fatalf("paginated envelope mismatch: %s", w.Body.String())
	}
}

func TestWrapBizError(t *testing.T) {
	w := run(t, response.Wrap(func(c *gin.Context) error {
		return errs.ErrNotFound
	}))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	var e env
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("response is not a unified envelope: %v", err)
	}
	if e.Code != errs.CodeNotFound || e.Msg != errs.ErrNotFound.Msg {
		t.Fatalf("envelope mismatch: %s", w.Body.String())
	}
}

func TestWrapWrappedBizError(t *testing.T) {
	// errors.As must unwrap the fmt.Errorf %w wrapper
	inner := errs.Wrap(errors.New("duplicate entry"), errs.CodeParam, "username already exists")
	w := run(t, response.Wrap(func(c *gin.Context) error {
		return fmt.Errorf("create user: %w", inner)
	}))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	var e env
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("response is not a unified envelope: %v", err)
	}
	if e.Msg != "username already exists" {
		t.Fatalf("msg = %q, want username already exists", e.Msg)
	}
}

func TestWrapUnknownError(t *testing.T) {
	w := run(t, response.Wrap(func(c *gin.Context) error {
		return errors.New("db: connection refused")
	}))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), "connection refused") {
		t.Fatalf("internal error leaked into the response: %s", w.Body.String())
	}
	var e env
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("response is not a unified envelope: %v", err)
	}
	if e.Code != errs.CodeInternal || e.Msg != errs.ErrInternal.Msg {
		t.Fatalf("envelope mismatch: %s", w.Body.String())
	}
}

func TestWrapMissingResponse(t *testing.T) {
	// when a handler forgets to write a response, fall back to 500 to avoid hanging the request
	w := run(t, response.Wrap(func(c *gin.Context) error {
		return nil
	}))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestWrapSuccessPassThrough(t *testing.T) {
	// response already written and nil returned: pass through normally
	w := run(t, response.Wrap(func(c *gin.Context) error {
		response.OK(c, nil)
		return nil
	}))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestFailCodeNormalization(t *testing.T) {
	// a mistakenly passed 2xx code is normalized to 500
	w := run(t, func(c *gin.Context) {
		response.Fail(c, 200, "misused success code")
	})

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

// bindRun simulates a real handler: POST JSON → ShouldBindJSON returns the error directly.
func bindRun(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", response.Wrap(func(c *gin.Context) error {
		var req struct {
			Name string `json:"name" binding:"required"`
			Age  int    `json:"age" binding:"gte=0"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			return err
		}
		response.OK(c, nil)
		return nil
	}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body)))
	return w
}

func TestWrapValidationErrorAs400(t *testing.T) {
	// a missing required field: expect 400 with the field name in msg, not a 500 fallback
	w := bindRun(t, `{"age":1}`)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	var e env
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if e.Code != errs.CodeParam || !strings.Contains(e.Msg, "Name") {
		t.Fatalf("code=%d msg=%q, expected a 400 message containing the field name Name", e.Code, e.Msg)
	}
}

func TestWrapJSONSyntaxErrorAs400(t *testing.T) {
	w := bindRun(t, `{bad json`)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	var e env
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if e.Code != errs.CodeParam {
		t.Fatalf("code=%d, want %d", e.Code, errs.CodeParam)
	}
}
