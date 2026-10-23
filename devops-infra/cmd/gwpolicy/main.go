// 命令 gwpolicy 用各业务服务自己的 bootstrap.yaml 渲染 Higress public ingress。
//
// 为什么需要它：匿名端点白名单写在每个服务自己的配置里（config/bootstrap.yaml 的
// gateway.expose 段），但网关侧的 ingress 是部署产物——手工维护两份，迟早只改一边，
// 表现是网关放行而应用层 401（或反过来）。本命令把配置渲染成 ingress，
// 清单就只有一份。
//
// 用法：
//
//	go run ./devops-infra/cmd/gwpolicy -root . -out deploy/higress/ingresses
//	go run ./devops-infra/cmd/gwpolicy -root . -out deploy/higress/ingresses -service devops-system
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/stanley760/devops/devops-infra/security/gateway"
)

func main() {
	root := flag.String("root", ".", "仓库根目录（用于扫描 <服务>/config/bootstrap.yaml）")
	out := flag.String("out", "deploy/higress/ingresses", "ingress 输出目录")
	service := flag.String("service", "", "只渲染指定服务（默认全部）")
	flag.Parse()

	services, err := discoverServices(*root)
	if err != nil {
		fail("扫描服务配置失败: %v", err)
	}
	if *service != "" {
		if _, ok := services[*service]; !ok {
			fail("没有找到服务 %q 的配置（可选：%v）", *service, keys(services))
		}
		services = map[string]string{*service: services[*service]}
	}
	if len(services) == 0 {
		fail("没有找到任何带 gateway.expose 的服务配置")
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail("创建输出目录失败: %v", err)
	}
	for _, name := range gateway.SortedServiceNames(keys(services)) {
		cfg, err := loadExpose(services[name])
		if err != nil {
			fail("读取 %s 的白名单失败: %v", name, err)
		}
		if cfg.IsEmpty() {
			// 没有声明暴露端点的服务（devops-id 等）不参与 ingress 渲染。
			fmt.Printf("==> %s 未声明 gateway.expose，跳过\n", name)
			continue
		}
		if err := cfg.Validate(); err != nil {
			fail("%s 的白名单不合法: %v", name, err)
		}
		data, err := gateway.RenderPublicIngress(name, cfg)
		if err != nil {
			fail("渲染 %s 失败: %v", name, err)
		}
		path := filepath.Join(*out, name+"-public.yaml")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			fail("写入 %s 失败: %v", path, err)
		}
		fmt.Printf("==> %s\n", path)
	}
}

// discoverServices 扫描 <root>/<服务>/config/bootstrap.yaml，返回服务名 → 配置路径。
//
// 服务名的约定：就是配置目录的上一级目录名（devops-system、devops-user…），
// 它同时是 ingress 资源名的前缀（<服务>-public）和 Nacos 服务名的前缀。
func discoverServices(root string) (map[string]string, error) {
	out := map[string]string{}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "devops-") {
			continue
		}
		cfgPath := filepath.Join(root, e.Name(), "config", "bootstrap.yaml")
		if _, err := os.Stat(cfgPath); err != nil {
			continue
		}
		out[e.Name()] = cfgPath
	}
	return out, nil
}

func loadExpose(path string) (gateway.ExposeConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return gateway.ExposeConfig{}, err
	}
	defer f.Close()
	return gateway.LoadExposeConfig(f)
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
