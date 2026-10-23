package gateway

import (
	"strings"
	"testing"
)

const sampleConfig = `
server:
  port: 9999
gateway:
  identity_source: gateway
  expose:
    prefix: /system
    host: api.devops.local
    destination: devops-system.DEVOPS-GROUP.public.nacos:9999
    endpoints:
      - path: /api/v1/auth/login
        reason: 登录：签发 access + refresh 令牌
      - path: /api/v1/auth/refresh
        reason: 刷新：access 过期后唯一的出路
`

func TestLoadExposeConfig(t *testing.T) {
	cfg, err := LoadExposeConfig(strings.NewReader(sampleConfig))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if cfg.Prefix != "/system" || cfg.Host != "api.devops.local" {
		t.Fatalf("前缀/host 不正确: %+v", cfg)
	}
	if len(cfg.Endpoints) != 2 {
		t.Fatalf("端点数 = %d，期望 2", len(cfg.Endpoints))
	}
	paths := cfg.ExemptPaths()
	want := []string{"/system/api/v1/auth/login", "/system/api/v1/auth/refresh"}
	for i, w := range want {
		if paths[i] != w {
			t.Errorf("第 %d 条豁免路径 = %s，期望 %s", i, paths[i], w)
		}
	}
}

func TestLoadExposeConfigRejectsBadInput(t *testing.T) {
	cases := map[string]string{
		"缺 host":        "gateway:\n  expose:\n    prefix: /a\n    destination: d\n    endpoints: []\n",
		"缺 destination": "gateway:\n  expose:\n    prefix: /a\n    host: h\n    endpoints: []\n",
		"路径不以 / 开头":     "gateway:\n  expose:\n    prefix: /a\n    host: h\n    destination: d\n    endpoints:\n      - path: api/v1/login\n",
		"端点重复":          "gateway:\n  expose:\n    prefix: /a\n    host: h\n    destination: d\n    endpoints:\n      - path: /x\n      - path: /x\n",
		"前缀不以 / 开头":     "gateway:\n  expose:\n    prefix: a\n    host: h\n    destination: d\n    endpoints: []\n",
	}
	for name, body := range cases {
		cfg, err := LoadExposeConfig(strings.NewReader(body))
		if err != nil {
			continue // 解析本身失败也算拒绝
		}
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s：应当报错", name)
		}
	}
}

// 没有 gateway.expose 段的服务应当被跳过，而不是报错。
func TestLoadExposeConfigEmpty(t *testing.T) {
	cfg, err := LoadExposeConfig(strings.NewReader("server:\n  port: 9990\n"))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if !cfg.IsEmpty() {
		t.Fatal("没有 gateway.expose 段时应为空")
	}
}

func TestPathMatchesTemplate(t *testing.T) {
	cases := []struct {
		tmpl, concrete string
		want           bool
	}{
		{"/user/api/v1/oauth/{provider}/callback", "/user/api/v1/oauth/github/callback", true},
		{"/user/api/v1/oauth/{provider}/callback", "/user/api/v1/oauth/wechat/callback", true},
		{"/user/api/v1/oauth/{provider}/callback", "/user/api/v1/oauth/bind/start", false},
		{"/user/api/v1/oauth/{provider}/callback", "/user/api/v1/oauth/github/callback/extra", false},
		{"/system/api/v1/auth/login", "/system/api/v1/auth/login", true},
		{"/system/api/v1/auth/login", "/system/api/v1/auth/logout", false},
	}
	for _, c := range cases {
		if got := PathMatchesTemplate(c.tmpl, c.concrete); got != c.want {
			t.Errorf("PathMatchesTemplate(%q, %q) = %v，期望 %v", c.tmpl, c.concrete, got, c.want)
		}
	}
}

func TestCheckExpose(t *testing.T) {
	cfg, err := LoadExposeConfig(strings.NewReader(sampleConfig))
	if err != nil {
		t.Fatal(err)
	}
	ops := []ExposedOp{
		{Path: "/system/api/v1/auth/login", Method: "POST", Anonymous: true},
		{Path: "/system/api/v1/auth/refresh", Method: "POST", Anonymous: true},
		{Path: "/system/api/v1/auth/logout", Method: "POST"},
		{Path: "/system/api/v1/users", Method: "GET"},
	}
	values := map[string][]string {
		"provider":{"github", "google"},
	}
	if err := CheckExpose(cfg, ops, values); err != nil {
		t.Fatalf("白名单与实际接口一致时不该报错: %v", err)
	}

	// 少一个匿名端点 → 白名单里的条目落空，必须报错。
	if err := CheckExpose(cfg, ops[:1], values); err == nil {
		t.Error("白名单里有、实际没有的匿名端点必须报错")
	}

	// 多一个匿名端点 → 白名单里没有它，也必须报错。
	extra := append(append([]ExposedOp{}, ops...), ExposedOp{
		Path: "/system/api/v1/auth/tenants", Method: "GET", Anonymous: true,
	})
	if err := CheckExpose(cfg, extra, values); err == nil {
		t.Error("实际是匿名、白名单里没有的端点必须报错")
	}
}

func TestRenderPublicIngress(t *testing.T) {
	cfg, err := LoadExposeConfig(strings.NewReader(sampleConfig))
	if err != nil {
		t.Fatal(err)
	}
	data, err := RenderPublicIngress("devops-system", cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	for _, want := range []string{
		"name: devops-system-public",
		"host: api.devops.local",
		"path: /system/api/v1/auth/login",
		"higress.io/destination: devops-system.DEVOPS-GROUP.public.nacos:9999",
		"# 登录：签发 access + refresh 令牌",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("渲染结果缺少 %q", want)
		}
	}
}
