# DevOps 平台安全评估报告

- **评估对象**：`D:/project/golang/devops`（Go 1.27 微服务：devops-user / devops-system / devops-id / devops-infra / devops-common / devops-frontend + deploy）
- **评估日期**：2026-10-22
- **评估方式**：静态代码审查 + 部署配置审查（人工逐文件核实，覆盖认证、授权、密钥、注入、暴露面、纵深防御）
- **知识库说明**：本会话环境未能接入腾讯内部安全知识库；本报告中的修复建议基于 OWASP、CWE、golang-jwt、Casbin、Nacos 等公开权威规范给出，落地前请以内部安全规范为准。

---

## 一、结论速览

**整体安全水位较高**。认证链路（RS256 非对称 + 强制 kid + 校验 iss/exp + Redis 会话吊销）、OAuth2 防 CSRF（state 一次性校验）、登录防爆破（captcha + 账户/IP 双维度限流 + 防枚举）、Casbin RBAC + 租户隔离、SQL 全参数化、JWT 私钥走占位符/Secret——这些关键控制都已扎实落地。

**主要风险集中在「部署与配置」层面**，而非业务代码：
- 🔴 **Nacos 未启用认证且端口发布到宿主机**（可直接读写配置、劫持服务发现）
- 🔴 **compose / 配置文件中明文提交弱口令**（`devopspwd`、`123456`、`rootpwd`、`admin`），且文档声称走 `${VAR}` 模板但文件实际硬编码
- 🟡 **开发/生产 compose 把 etcd、MySQL、Redis、Prometheus、Grafana、Nacos 等带数据服务直接发布到 `0.0.0.0`**，多数无认证
- 🟡 **Casbin RBAC 需在路由层逐端点确认 enforce 与 fail-closed**
- 🟡 **JWT 缺 `aud` 受众声明**、worker 缓存文件 `0o644` 世界可读

---

## 二、风险清单（按严重度排序）

### 🔴 [High] H1：Nacos 未启用认证且 8848 端口暴露到宿主机

- **位置**：`deploy/docker-compose.yml:65`（`NACOS_AUTH_ENABLE: "false"`）+ `:69-70`（`- "8848:8848"`）
- **类型**：CWE-306 Missing Authentication for Critical Function｜**置信度：高**
- **问题**：Nacos 是服务注册与配置中心（devops-id 通过 `nacos:///devops-id` 做服务发现，配置也由 Nacos 下发）。未启用认证 + 端口发布到宿主机意味着：任意能访问该端口的人可读写配置、注册/注销服务、篡改服务发现结果——可直接劫持后端流量或注入恶意配置。一旦宿主机暴露在不可信网络，等于把整个平台的交付链路拱手让人。
- **修复**：
  ```yaml
  nacos:
    environment:
      NACOS_AUTH_ENABLE: "true"
      NACOS_AUTH_TOKEN: "${NACOS_AUTH_TOKEN}"        # 强随机，经 secret 注入
      NACOS_AUTH_IDENTITY_KEY: "${NACOS_AUTH_KEY}"
    ports:
      - "127.0.0.1:8848:8848"   # 不暴露到 0.0.0.0；K8s 下用 NetworkPolicy 隔离
  ```
- **验证**：`curl http://<host>:8848/nacos/v1/ns/instance/list` 应返回 401。
- 📎 参考：Nacos 官方鉴权文档 https://nacos.io/docs/latest/manual/user/auth/

### 🔴 [High] H2：compose 与配置文件中明文提交弱口令（且与文档声明不一致）

- **位置**：
  - `deploy/config/system-prod.yaml:12` `password: "devopspwd"`（注释里写着 TODO 必须改，但值仍留在配置中）
  - `deploy/config/system-prod.yaml:16` Redis `password: "123456"`
  - `deploy/config/system-gray-prod.yaml:10,21` 同上弱口令
  - `deploy/config/system-dev.yaml:13,19` `devopspwd` / `123456`
  - `deploy/docker-compose.yml:88` Redis `--requirepass 123456`；`:42` `MYSQL_ROOT_PASSWORD: rootpwd`
  - `deploy/docker-compose-dev.yml:17,20,42,46` MySQL/Redis 弱口令
  - `deploy/docker-compose-dev.yml:215` `GF_SECURITY_ADMIN_PASSWORD=admin`
  - `deploy/higress/wasmplugins/cluster-key-rate-limit.yaml:15,27` `password: "123456"`
- **类型**：CWE-798 Use of Hard-coded Credentials / CWE-521 Weak Password｜**置信度：高**
- **问题**：
  1. 生产配置里直接写了弱口令（"devopspwd" 等），若这份配置被部署，生产数据库/Redis 用可被轻易猜解的密码直接暴露。
  2. `deploy/higress/README.md:616` 声称"密码通过 `.env` 注入、compose 用 `${VAR}` 引用"，**但实际的 compose / yaml 文件却是硬编码明文**——文档与现状不一致，是典型的安全隐患来源。
  3. 值得肯定：JWT 的 RS256 私钥在配置里是占位符 `REPLACE_WITH_RSA_PRIVATE_KEY_PEM`（`system-prod.yaml:30`），这部分处理正确；问题出在数据库/Redis/Grafana 口令。
- **修复**：所有口令改为经 K8s Secret / secrets 管理注入，仓库内只保留占位符或引用；生产口令用强随机（≥16 位）；对已有提交做密钥轮换。
- 📎 参考：OWASP ASVS V2.10 / CWE-798 https://cwe.mitre.org/data/definitions/798.html

### 🟡 [Medium] M1：大量带数据服务发布到 `0.0.0.0`（暴露面过大）

- **位置**：`deploy/docker-compose-dev.yml:26,45,142,188,203,219`；`deploy/docker-compose.yml:69,104,130`
- **类型**：CWE-668 Exposure of Resource to Wrong Sphere｜**置信度：高**
- **问题**：开发 compose 把 MySQL(3306)、Redis(6379)、etcd(2379)、Prometheus(9090)、Alertmanager(9093)、Grafana(3000) 全部映射到宿主机任意网卡。开发机若不在隔离网络（公司内网/云主机/VPN），这些带数据的服务等于裸奔；即便仅本机调试，误部署到云主机也会瞬间暴露。
- **修复**：开发环境一律绑定 `127.0.0.1`（`"127.0.0.1:3306:3306"`）；生产仅经网关/内网访问，用 K8s NetworkPolicy 收敛。

### 🟡 [Medium] M2：etcd 无认证且监听 `0.0.0.0` / 端口发布

- **位置**：`deploy/docker-compose.yml:149`（`--listen-client-urls=http://0.0.0.0:2379`）；`deploy/docker-compose-dev.yml:138,142`
- **类型**：CWE-306｜**置信度：高**
- **问题**：devops-id 用 etcd 存 worker 槽位/租约缓存。etcd 默认无认证，dev 还把 2379 发布到宿主机——容器内或宿主机的任意进程可读写 ID 生成状态，存在投毒/拒绝服务风险。
- **修复**：`--auth-token=...` 启用 etcd 认证；关闭 host 端口发布；仅 devops-id 同网络可达。

### 🟡 [Medium] M3：Casbin RBAC 需在路由层逐端点确认 enforce 与 fail-closed

- **位置**：`devops-infra/security/authz/authorizer.go`（基础设施已就绪）；需核对 `devops-system/internal/router` 各 handler
- **类型**：CWE-862 Missing Authorization｜**置信度：中**
- **问题**：RBAC 底座（Casbin + gorm-adapter + 租户 `domain` 隔离）设计正确。但静态层面无法确认**每一个敏感 handler 都调用了 `Enforce(subject, domain, permission)`**，以及 `Enforce` 返回 `(false, err)` 时调用方是否**一律拒绝**（fail-closed）。任何漏加 enforce 的端点都会成为越权缺口。
- **修复**：引入「路由级权限中间件」（`RequirePermission(perm)`），缺省拒绝；对 `Enforce` 错误统一按拒绝处理；补充回归测试覆盖「无权限返回 403」。
- 📎 参考：Casbin 官方 https://casbin.org/docs/

### 🟡 [Medium] M4：JWT 缺少 `aud`（受众）声明

- **位置**：`devops-infra/security/jwt/manager.go:50-58`（claims 未设 `Audience`）；`ParseAccess` 也未 `WithAudience`
- **类型**：CWE-345 / CWE-617 Insufficient Verification｜**置信度：低-中**
- **问题**：多个内部服务若共享同一 issuer（`devops`），任一服务签发的 access token 可被其它服务接受。加上 `aud` 并把校验收紧到本服务，可缩小令牌复用面。
- **修复**：签发时设 `Audience: []string{"devops-system"}`，解析时 `jwtlib.WithAudience("devops-system")`。

### 🟡 [Medium] M5：worker 缓存文件 `0o644` 世界可读 + bizTag 路径校验

- **位置**：`devops-id/internal/infrastructure/workerfile/workerfile.go:92`（`0o644`）；目录由 `root/leafName/bizTag` 拼接（`:21-22`）
- **类型**：CWE-377 / CWE-22｜**置信度：低**
- **问题**：缓存文件权限过宽（同机其它用户可读）；`bizTag` 由调用方传入，若未来来源扩展到不可信输入，可能造成路径穿越。当前 bizTag 来自内部配置，风险低，但属防御纵深缺口。
- **修复**：写入 `0o600`；对 `bizTag` 做白名单校验（仅允许 `[A-Za-z0-9_-]`，拒绝 `/` 与 `..`）。

### 🟡 [Medium] M6：监控栈（Prometheus / Alertmanager / Grafana）无鉴权暴露

- **位置**：`deploy/docker-compose-dev.yml:179-221`
- **类型**：CWE-200 Information Exposure｜**置信度：中**
- **问题**：Prometheus/Alertmanager 默认无鉴权；Grafana 用 `admin/admin`。指标接口会泄露内部主机名、路径、标签、业务 QPS 等，便于攻击者侦察；Grafana 弱口令可被直接登入。
- **修复**：监控组件不发布到公网，经反向代理加 Basic Auth 或 mTLS；Grafana 强口令 + 关闭匿名访问。

---

## 三、低风险 / 待确认项（Low）

| # | 项 | 位置 | 说明 |
|---|----|------|------|
| L1 | 口令最小长度仅 8 | `devops-user/.../service.go:178` | 建议提至 12；bcrypt cost=10 可接受 |
| L2 | `deploy/higress;D/` 空残留目录 | `deploy/` | 疑似误操作产生的重复目录，建议清理以免混淆部署/藏污 |
| L3 | 登录 IP 限流依赖 `clientIP` | `service.go:246` | 若 `X-Forwarded-For` 被信任且可伪造，IP 维度限流可能被绕过；账户维度仍有效。确认 `trusted_proxies` 仅含真实网关节点 |
| L4 | OAuth 回调 / 密码重置端点限流 | `devops-user/internal/router` | 登录已覆盖双维度限流，需确认回调与找回密码端点同样限流 |
| L5 | 注册邮箱/手机可选且未强制校验 | `service.go:208-213` | 若找回密码依赖邮箱，未验证邮箱=可绑定他人邮箱；需核对找回密码流程是否校验 verified |
| L6 | 匿名端点/白名单 | 各服务 `config/bootstrap.yaml` 的 `gateway.expose` 段 | **已整改（2026-10-23）**：白名单下沉到每个服务自己的 bootstrap.yaml（环境无关，本地与容器共用同一份，不会按环境分裂成多份），Higress public ingress 由它渲染（just gen-ingress），各服务 router 测试与实际注册接口双向比对（自动化断言已补上）。业务配置里的豁免入口**已彻底移除**（`exempt_paths` → `extra_exempt_paths` → 删除）：只存在于应用层的豁免不会让接口对外多暴露（网关那层仍要 JWT），只等于「跳过应用层二次校验」，且直连容器绕过网关时该路径裸奔。整改期间发现并修复两处真实漂移：devops-user 的 `oauth-providers` 缺匿名声明、OAuth 回调不在应用层豁免清单（生产 `require_jwt: true` 下会 401，OAuth 登录实际不可用） |

---

## 四、已落实的优良安全控制（确认无问题）

- ✅ **JWT 实现健壮**：RS256 非对称、解析时强制 `SigningMethodRS256`（拒绝 HS256 算法混淆）、强制 `kid`、校验 `iss` 与 `exp`；后端经 `middleware.Authenticate` 再次验签 + 查 Redis 会话状态实现吊销（`devops-infra/security/middleware/authenticate.go:50-68`）。
- ✅ **双重网关防护**：Higress jwt-auth 插件 + 应用层 `gateway.Enforce`（`require_jwt: true` 于 prod，且无 verifier 时 fail-closed）。
- ✅ **OAuth2 防 CSRF**：`state` 一次性校验（Redis `GETDEL`），绑定 provider，防「A 的 state 用在 B 的回调」；Google 校验 `aud`/`iss`（`provider.go:247-252`）。
- ✅ **登录防爆破设计周密**：captcha → 账户/IP 双维度限流、失败计数、统一错误防用户名枚举、bcrypt 且处理 >72 字节边界（`service.go:220-278`）。
- ✅ **密码存储安全**：bcrypt + 长度校验（`password.go`、`auth.go` 等），无明文/弱哈希（无 md5/sha1）。
- ✅ **SQL 全参数化**：生产查询均走 GORM 参数化；仅 `dbinit` 执行打包的静态 schema 脚本（非用户输入），无拼接注入。
- ✅ **配置密钥管理方向正确**：JWT 私钥在仓库中为占位符，生产走 K8s Secret（`deploy/k8s/devops.yaml` Secret 设计）。

---

## 五、修复优先级路线图

1. **立即（阻塞级）**：H2 所有弱口令改为 Secret 注入并轮换；H1 给 Nacos 开启认证并收敛端口。
2. **本周**：M1/M2 收敛 etcd/MySQL/Redis/Prometheus/Grafana 的 host 发布；M3 补齐路由级 enforce + fail-closed 回归测试；M6 监控栈加鉴权/隔离。
3. **下个迭代**：M4 加 `aud`；M5 文件权限 `0o600` + bizTag 白名单；L1 口令长度提至 12。
4. **长期**：L3-L6 逐项确认并完成自动化安全断言；建立 CI 密钥扫描（gitleaks）与 compose 端口/认证 lint。

---

## 六、参考来源

- OWASP Top 10 / ASVS：https://owasp.org/www-project-top-ten/ ｜ https://owasp.org/www-project-application-security-verification-standard/
- CWE：https://cwe.mitre.org/
- golang-jwt 最佳实践（算法固定、kid 校验）：https://github.com/golang-jwt/jwt
- Casbin RBAC：https://casbin.org/docs/
- Nacos 鉴权：https://nacos.io/docs/latest/manual/user/auth/
- etcd 安全：https://etcd.io/docs/v3.5/op-guide/authentication/
