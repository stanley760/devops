// P3 verification integration test: BaseModel ID assignment hook + idclient prefetch buffer, end to end.
// Depends on local MySQL (devops database) and the devops-id service (:9090); skipped when unreachable.
package model_test

import (
	"context"
	"fmt"
	"github.com/stanley760/devops/devops-id/pkg/idclient"
	infracfg "github.com/stanley760/devops/devops-infra/config"
	client "github.com/stanley760/devops/devops-infra/database/mysql"
	grpcinfra "github.com/stanley760/devops/devops-infra/grpc"
	"github.com/stanley760/devops/devops-system/internal/config"
	"github.com/stanley760/devops/devops-system/internal/model"
	"testing"
	"time"
)

// setup loads config → connects to MySQL → assembles the ID generator client; Skips if any dependency is unreachable.
func setup(t *testing.T) {
	t.Helper()

	if err := infracfg.Init("../../config/config.yaml"); err != nil {
		t.Skipf("failed to load config, skipping integration test: %v", err)
	}
	var cfg config.Config
	if err := infracfg.LoadInto(&cfg); err != nil {
		t.Skipf("failed to parse config, skipping integration test: %v", err)
	}
	if err := client.Init(cfg.Mysql); err != nil {
		t.Skipf("MySQL unreachable, skipping integration test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := grpcinfra.Dial(ctx, cfg.IDClient.Endpoint, 2*time.Second)
	if err != nil {
		t.Skipf("devops-id dial failed, skipping integration test: %v", err)
	}
	idc := idclient.New(conn, idclient.BufferConfig{
		Batch:       100,
		LowWater:    0.2,
		CallTimeout: 3 * time.Second,
	})
	if err := idc.Ping(ctx); err != nil {
		t.Skipf("devops-id health check failed, skipping integration test: %v", err)
	}
	model.SetIDGenerator(idc)
	t.Cleanup(func() { _ = conn.Close() })
}

// TestBeforeCreateGeneratesID: creating a user → the primary key is assigned by devops-id (IDs increase by batch).
func TestBeforeCreateGeneratesID(t *testing.T) {
	setup(t)
	gdb := client.Get()

	ts := time.Now().UnixNano()
	u1 := &model.User{
		BaseTenantModel: model.BaseTenantModel{TenantID: 1},
		Username:        fmt.Sprintf("it_seq_%d_a", ts),
		Password:        "$2a$10$placeholder",
		Nickname:        "集成测试A",
		Status:          0,
	}
	u2 := &model.User{
		BaseTenantModel: model.BaseTenantModel{TenantID: 1},
		Username:        fmt.Sprintf("it_seq_%d_b", ts),
		Password:        "$2a$10$placeholder",
		Nickname:        "集成测试B",
		Status:          0,
	}
	t.Cleanup(func() {
		gdb.Unscoped().Where("username LIKE ?", fmt.Sprintf("it_seq_%d%%", ts)).Delete(&model.User{})
	})

	if err := gdb.Create(u1).Error; err != nil {
		t.Fatalf("failed to create user A: %v", err)
	}
	if err := gdb.Create(u2).Error; err != nil {
		t.Fatalf("failed to create user B: %v", err)
	}

	if u1.ID <= 0 {
		t.Fatalf("user A did not get an assigned ID: %d", u1.ID)
	}
	// within the same prefetch buffer: batch-incrementing (consecutive like 1001, 1002 or increasing across batches)
	if u2.ID <= u1.ID {
		t.Fatalf("IDs not increasing: u1=%d u2=%d", u1.ID, u2.ID)
	}
	t.Logf("ID assignment succeeded: u1=%d u2=%d", u1.ID, u2.ID)
}

// TestBizTagOf: model type → biz_tag derivation.
func TestBizTagOf(t *testing.T) {
	cases := []struct {
		model any
		tag   string
	}{
		{&model.User{}, "user"},
		{&model.Role{}, "role"},
		{&model.Menu{}, "menu"},
		{&model.Dept{}, "dept"},
		{&model.Post{}, "post"},
		{&model.Tenant{}, "tenant"},
		{&model.LoginLog{}, "login_log"},
	}
	for _, c := range cases {
		tag, err := model.BizTagOf(c.model)
		if err != nil || tag != c.tag {
			t.Errorf("BizTagOf(%T) = %q, %v; want %q, nil", c.model, tag, err, c.tag)
		}
	}
}
