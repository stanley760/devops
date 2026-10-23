package gateway

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ExposeEndpoint 是「网关放行（免 JWT）」的一个端点。
type ExposeEndpoint struct {
	// Path 是端点路径：以 / 开头。
	Path string `mapstructure:"path" yaml:"path"`
	// Absolute 为 true 时 Path 已是完整路径（不拼服务前缀）——用于主机级端点
	// （如 /.well-known/jwks.json，它挂在服务自己的域名上，与 /system、/user 无关）。
	Absolute bool `mapstructure:"absolute" yaml:"absolute"`
	// Reason 说明为什么必须匿名。渲染 ingress 时作为该路径的注释输出，
	// 这样「为什么放行」跟着配置走，不会和代码注释脱节。
	Reason string `mapstructure:"reason" yaml:"reason"`
}

// ExposeConfig 是一个服务的网关暴露契约。
//
// 它写在**各业务服务自己的 bootstrap.yaml** 里（gateway.expose 段）——
// 每个服务一份，是该服务「哪些接口可以匿名访问」的唯一声明。
//
// 一次声明，两处消费：
//   - 应用层：jwt 模式（前面没有网关的环境）下，它是粗粒度守卫的豁免表；
//   - 网关侧：Higress 的 public ingress 由它渲染（just gen-ingress），
//     所以「网关放行的路径」与「应用层豁免的路径」不会各写一份而漂移；
//   - 各服务的路由测试再拿它和实际注册的接口比对，防止白名单与 API 面脱节。
type ExposeConfig struct {
	// Prefix 是服务的 API 前缀（如 /system），端点路径拼在它后面。
	Prefix string `mapstructure:"prefix" yaml:"prefix"`
	// Host 是该服务在网关上的域名（渲染 public ingress 用）。
	Host string `mapstructure:"host" yaml:"host"`
	// Destination 是 Higress 的后端地址（渲染 public ingress 用）。
	Destination string `mapstructure:"destination" yaml:"destination"`
	// Endpoints 是需要匿名访问的端点。
	Endpoints []ExposeEndpoint `mapstructure:"endpoints" yaml:"endpoints"`
}

// FullPath 返回端点的完整路径（绝对路径原样返回，其余拼上服务前缀）。
func (c ExposeConfig) FullPath(e ExposeEndpoint) string {
	if e.Absolute {
		return e.Path
	}
	return c.Prefix + e.Path
}

// ExemptPaths 返回需要免 JWT 校验的完整路径清单。
func (c ExposeConfig) ExemptPaths() []string {
	out := make([]string, 0, len(c.Endpoints))
	for _, e := range c.Endpoints {
		out = append(out, c.FullPath(e))
	}
	return out
}

// Validate 校验白名单本身：路径格式、重复项、必要字段。
//
// 白名单写错一个字符就等于在网关上开了一个洞，所以这些都在加载时就挡掉，
// 不给到运行时。
func (c ExposeConfig) Validate() error {
	if c.Prefix != "" && !strings.HasPrefix(c.Prefix, "/") {
		return fmt.Errorf("服务前缀 %q 必须以 / 开头", c.Prefix)
	}
	if c.Host == "" {
		return fmt.Errorf("缺少 host（渲染 public ingress 用）")
	}
	if c.Destination == "" {
		return fmt.Errorf("缺少 destination（渲染 public ingress 用）")
	}
	seen := make(map[string]struct{}, len(c.Endpoints))
	for _, e := range c.Endpoints {
		if e.Path == "" {
			return fmt.Errorf("端点路径为空")
		}
		if !strings.HasPrefix(e.Path, "/") {
			return fmt.Errorf("端点路径 %q 必须以 / 开头", e.Path)
		}
		if strings.ContainsAny(e.Path, " \t") {
			return fmt.Errorf("端点路径 %q 不能包含空白字符", e.Path)
		}
		if _, dup := seen[e.Path]; dup {
			return fmt.Errorf("端点 %q 重复", e.Path)
		}
		seen[e.Path] = struct{}{}
	}
	return nil
}

// LoadExposeConfig 从一份业务配置 YAML 中取出 gateway.expose 段。
//
// 业务配置里还有很多别的段（server / nacos / database…），所以这里只声明
// 需要的结构、用宽松解码——不认识的键照常忽略。
//
// 只做解析，不做校验：没有 gateway.expose 段的服务（devops-id 等）应当被跳过，
// 而不是报错。校验由调用方按需触发（Validate）。
func LoadExposeConfig(r io.Reader) (ExposeConfig, error) {
	var doc struct {
		Gateway struct {
			Expose ExposeConfig `yaml:"expose"`
		} `yaml:"gateway"`
	}
	if err := yaml.NewDecoder(r).Decode(&doc); err != nil {
		return ExposeConfig{}, fmt.Errorf("解析 gateway.expose 失败: %w", err)
	}
	return doc.Gateway.Expose, nil
}

// IsEmpty 表示这个服务没有声明任何网关暴露端点（不参与 ingress 渲染）。
func (c ExposeConfig) IsEmpty() bool {
	return len(c.Endpoints) == 0 && c.Host == "" && c.Destination == ""
}

// ExposedOp 描述一个已注册端点及其是否匿名。
type ExposedOp struct {
	// Path 是 OpenAPI 里的路径，可能带 {param} 模板。
	Path string
	// Method 是 HTTP 方法。
	Method string
	// Anonymous 表示该端点声明为免鉴权。
	Anonymous bool
}

// CheckExpose 校验「白名单」与「实际注册的接口」没有漂移。
//
// 两个方向都要查：
//   - 白名单里有、实际却没有匿名端点：端点改名或改成需要登录了，白名单会失效
//     或变成一个洞；
//   - 实际是匿名端点、白名单里却没有：网关/守卫会把它拦下来，用户拿到 401，
//     而清单看起来是「配好了」的——这类漂移最难查。
//
// **模板端点必须穷举覆盖**：像 /oauth/{provider}/callback 这样的路径，路由只注册
// 一条，但网关侧只能按字面前缀放行（Ingress 的 pathType: Prefix 不支持 {} 模板），
// 所以白名单必须逐个取值写全。如果只按「有一条能匹配上」判定，那么删掉白名单里
// 任意一个 provider 的条目都不会被发现——而新增 provider 时应用层路由自动生效
// （模板匹配任意取值），白名单漏了它，网关就会拦掉那个回调，用户表现为
// 「点了第三方登录，跳回来就 401」。
//
// values 给出模板参数的已知取值集合（如 {"provider": ["github","google"]}）：
// 白名单必须覆盖每一个取值才算同步。字面量端点不受影响。
//
// 绝对路径（主机级端点，如 /.well-known/jwks.json）不在这里比对：它们不是
// huma 注册的接口，不出现在 OpenAPI 文档里；由各服务自己断言（见 router 测试）。
//
// ops 由调用方从 OpenAPI 文档解析（各服务的 router 测试负责）。
func CheckExpose(cfg ExposeConfig, ops []ExposedOp, values map[string][]string) error {
	for _, e := range cfg.Endpoints {
		if e.Absolute {
			continue
		}
		full := cfg.FullPath(e)
		found := false
		for _, op := range ops {
			if op.Anonymous && PathMatchesTemplate(op.Path, full) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("白名单里的 %s 在实际注册的接口中找不到对应的匿名端点", full)
		}
	}
	for _, op := range ops {
		if !op.Anonymous {
			continue
		}
		params := TemplateParams(op.Path)
		if len(params) == 0 {
			// 字面量端点：白名单里必须原样存在。
			if !cfg.has(op.Path) {
				return fmt.Errorf("端点 %s (%s) 声明为匿名，但白名单里没有它", op.Path, op.Method)
			}
			continue
		}
		// 模板端点：每个参数取值都要在白名单里出现。
		for _, p := range params {
			vals, ok := values[p]
			if !ok || len(vals) == 0 {
				return fmt.Errorf("端点 %s (%s) 是模板端点，但没有提供参数 %q 的取值集合，无法校验白名单是否穷举覆盖", op.Path, op.Method, p)
			}
			for _, v := range vals {
				concrete := strings.Replace(op.Path, "{"+p+"}", v, 1)
				if !cfg.has(concrete) {
					return fmt.Errorf("端点 %s (%s) 是模板端点，白名单未覆盖取值 %s（缺少 %s）", op.Path, op.Method, v, concrete)
				}
			}
		}
	}
	return nil
}

// has 判断白名单里是否原样包含某个完整路径。
func (c ExposeConfig) has(fullPath string) bool {
	for _, e := range c.Endpoints {
		if c.FullPath(e) == fullPath {
			return true
		}
	}
	return false
}

// TemplateParams 返回路径里的模板参数名（如 /a/{provider}/b → ["provider"]）。
func TemplateParams(path string) []string {
	var out []string
	for _, seg := range strings.Split(path, "/") {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") && len(seg) > 2 {
			out = append(out, seg[1:len(seg)-1])
		}
	}
	return out
}

// PathMatchesTemplate 判断一个具体路径是否满足某个路由模板。
//
// 模板里的 {param} 匹配任意一段（不含 /）。用于把 OpenAPI 的模板路径
// （/user/api/v1/oauth/{provider}/callback）和白名单里的具体路径
// （/user/api/v1/oauth/github/callback）对上。
func PathMatchesTemplate(tmpl, concrete string) bool {
	tp := strings.Split(tmpl, "/")
	cp := strings.Split(concrete, "/")
	if len(tp) != len(cp) {
		return false
	}
	for i, seg := range tp {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			if cp[i] == "" {
				return false
			}
			continue
		}
		if seg != cp[i] {
			return false
		}
	}
	return true
}

// ingressHeader 是渲染 public ingress 时输出的说明。
//
// 这段解释的是 Higress 的机制约束（jwt-auth 不支持按路径匹配，所以
// 匿名端点必须单独走一条 ingress），与具体服务无关；但里面举的例子要跟着
// 服务走，所以按服务参数化。
func ingressHeader(name string, cfg ExposeConfig) string {
	example := cfg.Prefix + "/api/v1/auth/login"
	if len(cfg.Endpoints) > 0 {
		example = cfg.FullPath(cfg.Endpoints[0])
	}
	return fmt.Sprintf(`# 由业务服务的 bootstrap.yaml 渲染，请勿手工编辑。
# 白名单来源：%s/config/bootstrap.yaml 的 gateway.expose 段
# 重新生成：just gen-ingress
#
# 匿名端点专用路由：承载所有豁免 jwt-auth 插件的端点。
#
# 为什么必须单独一条 ingress：jwt-auth 的 matchRules 只支持 ingress / domain /
# service 三个维度，**不支持按 URL 路径匹配**。主路由 %s 挂在
# %s 前缀下，而登录接口 %s 正好落在这个前缀里——
# 把插件挂在主路由上就会强制校验登录令牌，而此时调用方还没有令牌，
# 结果是没人能登录。
#
# 解法：把匿名端点拆到独立 ingress；Higress 按**最长前缀匹配**路由，
# 从而绕开挂在主路由上的 jwt-auth 插件。
#
# 路径必须写全：前缀匹配下 %s 命中本路由；
# 但不能合并成 %s/api/v1/auth，否则该前缀下其他接口（如修改密码）
# 也会被一并放行。
`, name, name, cfg.Prefix, example, example, cfg.Prefix)
}

// RenderPublicIngress 用白名单渲染某个服务的 Higress public ingress。
//
// 输出的注释直接来自白名单里的 reason：清单和「为什么放行」写在同一处，
// 评审时看 ingress 就能知道每条路径为什么免鉴权。
func RenderPublicIngress(name string, cfg ExposeConfig) ([]byte, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	var b strings.Builder
	b.WriteString(ingressHeader(name, cfg))
	b.WriteString(`apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: `)
	b.WriteString(name)
	b.WriteString(`-public
  namespace: higress-system
  annotations:
    # 匿名端点：凭据校验由登录接口自身处理，静态身份头机制已移除，无需任何身份注解。
    higress.io/destination: `)
	b.WriteString(cfg.Destination)
	b.WriteString(`
  labels:
    higress.io/resource-definer: higress
spec:
  ingressClassName: higress
  rules:
    - host: `)
	b.WriteString(cfg.Host)
	b.WriteString(`
      http:
        paths:
`)
	writePath := func(path, reason string) {
		if reason != "" {
			for _, line := range strings.Split(strings.TrimRight(reason, "\n"), "\n") {
				b.WriteString("          # ")
				b.WriteString(line)
				b.WriteString("\n")
			}
		}
		b.WriteString(`          - path: `)
		b.WriteString(path)
		b.WriteString(`
            pathType: Prefix
            backend:
              resource:
                apiGroup: networking.higress.io
                kind: McpBridge
                name: default
`)
	}
	for _, e := range cfg.Endpoints {
		writePath(cfg.FullPath(e), e.Reason)
	}
	return []byte(b.String()), nil
}

// SortedServiceNames 对服务名排序，保证生成顺序稳定。
func SortedServiceNames(names []string) []string {
	out := append([]string{}, names...)
	sort.Strings(out)
	return out
}
