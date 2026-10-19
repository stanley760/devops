// Package service 承载 C 端业务用例。
//
// 分层：本包只做「业务规则 + 参数校验」，SQL 全在 internal/dao，
// handler（internal/api/v1）不感知本包。这样换存储实现时本包接口不变。
package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"

	"github.com/stanley760/devops/devops-infra/errs"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/stanley760/devops/devops-user/internal/auth"
	"github.com/stanley760/devops/devops-user/internal/dao"
	"github.com/stanley760/devops/devops-user/internal/loginlock"
	"github.com/stanley760/devops/devops-user/internal/model"
)

// authCounter 业务指标：登录/注册尝试计数（auth_login / auth_register → auth_*_total）。
// 标签均为有界集：result=success|failure，code=失败业务码（成功恒 200）。
// 惰性创建：首次调用时全局 MeterProvider 已由 observability.Init 装配完毕；
// 未初始化（单测/探针）自动落到 noop 仪表。指标故障不影响业务返回。
var (
	authLoginCounter = sync.OnceValue(func() metric.Int64Counter {
		c, err := otel.Meter("github.com/stanley760/devops/devops-user/service").Int64Counter(
			"auth_login",
			metric.WithDescription("登录尝试计数；result=success|failure，code 为失败业务码（成功=200）"),
		)
		if err != nil {
			otel.Handle(err)
		}
		return c
	})
	authRegisterCounter = sync.OnceValue(func() metric.Int64Counter {
		c, err := otel.Meter("github.com/stanley760/devops/devops-user/service").Int64Counter(
			"auth_register",
			metric.WithDescription("注册尝试计数；result=success|failure，code 为失败业务码（成功=200）"),
		)
		if err != nil {
			otel.Handle(err)
		}
		return c
	})
)

// authResultAttr 认证结果标签值。
func authResultAttr(err error) attribute.KeyValue {
	if err != nil {
		return attribute.String("result", "failure")
	}
	return attribute.String("result", "success")
}

// authCodeAttr 失败原因码：BizError 携带的有界业务码，非 BizError 按 500 归类。
// 失败的具体细节（密码错误/用户不存在/锁定等）走访问日志，不进指标标签控制基数。
func authCodeAttr(err error) attribute.KeyValue {
	return attribute.Int("code", authErrCode(err))
}

// authErrCode 认证结果业务码（成功 200；失败取 BizError 的 4xx/5xx，非业务错误按 500）。
func authErrCode(err error) int {
	if err == nil {
		return errs.CodeOK
	}
	var be *errs.BizError
	if errors.As(err, &be) && be.Code >= 400 && be.Code <= 599 {
		return be.Code
	}
	return errs.CodeInternal
}

// 业务错误（服务内定义，handler 感知不到）。
//
// ⚠️ 这些错误最终都要经 errs.ToHumaError 转成带状态码的信封，
// 直接 return 会被 huma 兜底成 500（见 api/v1 的包注释）。
var (
	// ErrInvalidCredentials 账号或密码错误。
	//
	// 「用户不存在」与「密码错误」**必须**返回同一个错误：区分二者会让攻击者
	// 据此枚举出哪些用户名存在。dao.GetUserByUsername 已按此约定返回本错误。
	ErrInvalidCredentials = errors.New("账号或密码错误")
	// ErrAccountDisabled 账号已封禁。
	ErrAccountDisabled = errors.New("账号已被封禁")
	// ErrNotSelf 试图操作非本人数据（C 端行级归属校验）。
	ErrNotSelf = errors.New("只能操作自己的数据")
)

// usernamePattern 登录名规则：以字母开头，3-32 位字母数字下划线。
//
// 与 database 内 uk_username 的唯一约束是两个层次：本正则给用户友好提示，
// 唯一索引保证并发下不会重名（竞态由 dao.CreateUser 兜底）。
var usernamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{2,31}$`)

// validGender 性别取值：0未知 1男 2女（与 devops-system 的 t_system_users 同约定）。
// 在 service 收口校验：API 层的 huma minimum/maximum 只挡直连 HTTP 的调用，
// 换别的入口（如三方自动建号）时此处仍兜底，避免脏值入库。
func validGender(g int8) bool { return g >= 0 && g <= 2 }

// Auth 认证用例（注册 / 登录 / 登出）。
type Auth struct {
	authM   *auth.Manager
	limiter *loginlock.Limiter
}

// NewAuth 构造认证用例。limiter 为 nil 时登录不加限流（仅测试用）。
func NewAuth(authM *auth.Manager, limiter *loginlock.Limiter) *Auth {
	return &Auth{authM: authM, limiter: limiter}
}

// RegisterInput 注册入参。
type RegisterInput struct {
	Username string
	Password string
	Nickname string
	Email    string
	Mobile   string
	Gender   int8
}

// TokenPair 双 token 签发结果（转发 auth.TokenPair，避免重复定义）。
type TokenPair = auth.TokenPair

// Register 创建账号并直接签发 access token（省去注册后立即再登录一次）。
// 业务指标：无论成败记一次 auth_register_total{result, code}，指标故障不影响注册。
func (s *Auth) Register(ctx context.Context, in RegisterInput) (TokenPair, error) {
	pair, err := s.register(ctx, in)
	authRegisterCounter().Add(ctx, 1, metric.WithAttributes(authResultAttr(err), authCodeAttr(err)))
	return pair, err
}

// register 注册主流程。
func (s *Auth) register(ctx context.Context, in RegisterInput) (TokenPair, error) {
	if !usernamePattern.MatchString(in.Username) {
		return TokenPair{}, errs.New(errs.CodeParam,
			"用户名须以字母开头，3-32 位字母数字下划线")
	}
	if len(in.Password) < 8 {
		return TokenPair{}, errs.New(errs.CodeParam, "密码至少 8 位")
	}
	if !validGender(in.Gender) {
		return TokenPair{}, errs.New(errs.CodeParam, "性别取值不合法（0未知 1男 2女）")
	}

	// 提前拦截以给出友好文案；并发竞态下仍以唯一索引为准。
	taken, err := dao.IsUsernameTaken(ctx, in.Username)
	if err != nil {
		return TokenPair{}, err
	}
	if taken {
		return TokenPair{}, errs.New(errs.CodeParam, "用户名已被注册")
	}

	hash, err := hashPassword(in.Password)
	if err != nil {
		if errors.Is(err, errPasswordTooLong) {
			return TokenPair{}, errs.New(errs.CodeParam, "密码过长（最多 72 字节）")
		}
		return TokenPair{}, errs.New(errs.CodeInternal, "口令加密失败")
	}
	u := &model.User{
		Username: in.Username,
		Password: hash,
		Nickname: in.Nickname,
		Gender:   in.Gender,
	}
	if in.Email != "" {
		u.Email = &in.Email
	}
	if in.Mobile != "" {
		u.Mobile = &in.Mobile
	}
	if err := dao.CreateUser(ctx, u); err != nil {
		return TokenPair{}, err
	}
	return s.issue(ctx, u.ID, u.Username)
}

// Login 校验账号密码并签发双 token。
//
// ⚠️ 「用户不存在」与「密码错误」**必须返回完全相同的文案**（含状态码），
// 否则攻击者可据此枚举出哪些用户名存在。dao 对不存在返回统一文案，
// 此处密码错误也用同一句——改动时两处须同步。
//
// 限流与锁定：校验**之前**检查、失败**之后**记录。
//   - 「用户不存在」也算一次失败——否则攻击者可无成本枚举用户名
//     （既不计数也不锁定）；
//   - 账号已封禁**不计**失败——不是用户输错，是账号状态问题，
//     计入会导致误封正常用户的IP 计数。
//   - 成功后清空该账号计数；IP 计数不清，否则正常用户偶发输错就永久触发 IP 限流。
//
// 业务指标：无论成败记一次 auth_login_total{result, code}，指标故障不影响登录。
func (s *Auth) Login(ctx context.Context, clientIP, username, password string) (TokenPair, error) {
	pair, err := s.login(ctx, clientIP, username, password)
	authLoginCounter().Add(ctx, 1, metric.WithAttributes(authResultAttr(err), authCodeAttr(err)))
	return pair, err
}

// login 登录主流程。
func (s *Auth) login(ctx context.Context, clientIP, username, password string) (TokenPair, error) {
	if err := s.checkLimit(ctx, clientIP, username); err != nil {
		return TokenPair{}, err
	}
	u, err := dao.GetUserByUsername(ctx, username)
	if err != nil {
		s.recordFail(ctx, clientIP, username)
		return TokenPair{}, err
	}
	if !checkPassword(password, u.Password) {
		s.recordFail(ctx, clientIP, username)
		return TokenPair{}, errs.New(errs.CodeUnauthorized, ErrInvalidCredentials.Error())
	}
	if u.Disabled {
		return TokenPair{}, errs.New(errs.CodeForbidden, ErrAccountDisabled.Error())
	}
	s.clearFail(ctx, username)
	return s.issue(ctx, u.ID, u.Username)
}

// Refresh 用 refresh token 换新双 token。
//
// 流程：先用 refresh 换出 access（内含身份）→ 查库确认账号未被封禁 →
// 用库中最新用户名重新签发（首次 Refresh 时 Manager 侧 username 为空）。
//
// 封禁时顺带作废该 refresh，否则封禁后仍能无限续期。
func (s *Auth) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	pair, err := s.authM.Refresh(ctx, refreshToken, "")
	if err != nil {
		return TokenPair{}, errs.New(errs.CodeUnauthorized, auth.ErrRefreshInvalid.Error())
	}
	claims, err := s.authM.Parse(pair.AccessToken)
	if err != nil {
		return TokenPair{}, errs.New(errs.CodeUnauthorized, auth.ErrRefreshInvalid.Error())
	}
	u, err := dao.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return TokenPair{}, err
	}
	if u.Disabled {
		_ = s.authM.RevokeRefresh(ctx, pair.RefreshToken)
		return TokenPair{}, errs.New(errs.CodeForbidden, ErrAccountDisabled.Error())
	}
	return s.issue(ctx, u.ID, u.Username)
}

// Logout 撤销当前会话与 refresh token。
//
// refresh 一并作废：否则登出后客户端仍可用 refresh 换出新 token，等于没登出。
func (s *Auth) Logout(ctx context.Context, claims *auth.AccessClaims, refreshToken string) error {
	if err := s.authM.Logout(ctx, claims); err != nil {
		return err
	}
	return s.authM.RevokeRefresh(ctx, refreshToken)
}

func (s *Auth) issue(ctx context.Context, userID int64, username string) (TokenPair, error) {
	pair, err := s.authM.Issue(ctx, userID, username)
	if err != nil {
		return TokenPair{}, errs.New(errs.CodeInternal, "签发令牌失败")
	}
	return pair, nil
}

// issueFor 与 issue 相同，语义上用于「刷新/换发」路径。
func (s *Auth) issueFor(ctx context.Context, userID int64, username string) (TokenPair, error) {
	return s.issue(ctx, userID, username)
}

// checkLimit 登录前置限流检查。
func (s *Auth) checkLimit(ctx context.Context, clientIP, username string) error {
	if s.limiter == nil {
		return nil
	}
	switch err := s.limiter.Check(ctx, clientIP, username); {
	case errors.Is(err, loginlock.ErrLocked):
		return errs.New(errs.CodeForbidden, "登录失败次数过多，账号已临时锁定，请 15 分钟后再试")
	case errors.Is(err, loginlock.ErrTooManyAttempts):
		return errs.New(errs.CodeTooManyRequests, "尝试过于频繁，请稍后再试")
	case err != nil:
		return errs.New(errs.CodeInternal, "登录限流检查失败")
	}
	return nil
}

// recordFail 记录一次登录失败。
func (s *Auth) recordFail(ctx context.Context, clientIP, username string) {
	if s.limiter != nil {
		s.limiter.Fail(ctx, clientIP, username)
	}
}

// clearFail 登录成功后清空该账号失败计数。
func (s *Auth) clearFail(ctx context.Context, username string) {
	if s.limiter != nil {
		s.limiter.Success(ctx, username)
	}
}

// Profile 用户资料用例（含改密、封禁）。
type Profile struct {
	// authM 用于改密/封禁后强制该用户全端下线。
	authM *auth.Manager
	// authLimiter 用于改密成功后清空登录失败计数；nil 时跳过。
	authLimiter *loginlock.Limiter
}

// NewProfile 构造资料用例。limiter 为 nil 时改密不清理失败计数。
func NewProfile(authM *auth.Manager, limiter *loginlock.Limiter) *Profile {
	return &Profile{authM: authM, authLimiter: limiter}
}

// Get 读取当前用户资料。
//
// claims 由 JWT 中间件注入，**不接受**客户端传入的用户 ID ——
// C 端的行级归属校验靠「身份只来自 token」实现，不靠参数过滤。
func (s *Profile) Get(ctx context.Context, claims *auth.AccessClaims) (model.PublicProfile, error) {
	if claims == nil {
		return model.PublicProfile{}, errs.ErrUnauthorized
	}
	u, err := dao.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return model.PublicProfile{}, err
	}
	return u.ToPublic(), nil
}

// UpdateInput 资料修改入参（指针字段：nil 表示不修改该字段）。
type UpdateInput struct {
	Nickname *string
	Email    *string
	Mobile   *string
	Gender   *int8
	Avatar   *string
}

// Update 修改当前用户资料。只能改自己的（身份来自 token，天然满足）。
func (s *Profile) Update(ctx context.Context, claims *auth.AccessClaims, in UpdateInput) (model.PublicProfile, error) {
	if claims == nil {
		return model.PublicProfile{}, errs.ErrUnauthorized
	}

	// 只把非 nil 字段放进 map：dao 用 map 更新，避免未传字段被零值覆盖。
	fields := make(map[string]any, 4)
	if in.Nickname != nil {
		fields["nickname"] = *in.Nickname
	}
	if in.Email != nil {
		fields["email"] = model.NormalizePtr(in.Email)
	}
	if in.Mobile != nil {
		fields["mobile"] = model.NormalizePtr(in.Mobile)
	}
	if in.Gender != nil {
		if !validGender(*in.Gender) {
			return model.PublicProfile{}, errs.New(errs.CodeParam, "性别取值不合法（0未知 1男 2女）")
		}
		fields["gender"] = *in.Gender
	}
	if in.Avatar != nil {
		fields["avatar"] = model.NormalizePtr(in.Avatar)
	}

	if err := dao.UpdateProfile(ctx, claims.UserID, fields); err != nil {
		return model.PublicProfile{}, err
	}
	return s.Get(ctx, claims)
}

// ChangePassword 修改自己的口令。
//
// 安全要求：
//   - 必须校验旧口令——否则拿到 token 的人可直接改密永久占据账号；
//   - 新旧相同直接拒绝；
//   - 校验通过后清空该账号的登录失败计数（可能正因连续输错被临时锁定）。
//
// **改密即全端下线**（默认行为）：改密成功后强制该用户所有设备重新登录。
// 理由：口令泄露是安全事件，攻击者可能已持有其他设备的 refresh；
// 只改本地会话会留下可用的入侵通道。
//
// keepCurrentRefresh 非空时保留当前设备（其余设备下线）——注意此时 access
// 侧仍会全踢（session 侧无 sid → refresh 反查），故当前设备也需重新登录，
// 只是它的 refresh 还在、可直接换新 access。**当前默认传空**，即全踢。
func (s *Profile) ChangePassword(ctx context.Context, claims *auth.AccessClaims, oldPassword, newPassword, keepCurrentRefresh string) error {
	if claims == nil {
		return errs.ErrUnauthorized
	}
	if len(newPassword) < 8 {
		return errs.New(errs.CodeParam, "新密码至少 8 位")
	}
	u, err := dao.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return err
	}
	if !checkPassword(oldPassword, u.Password) {
		return errs.New(errs.CodeUnauthorized, "原密码不正确")
	}
	if oldPassword == newPassword {
		return errs.New(errs.CodeParam, "新密码不能与原密码相同")
	}
	hashed, err := hashPassword(newPassword)
	if err != nil {
		if errors.Is(err, errPasswordTooLong) {
			return errs.New(errs.CodeParam, "密码过长（最多 72 字节）")
		}
		return errs.New(errs.CodeInternal, "口令加密失败")
	}
	if err := dao.UpdatePassword(ctx, claims.UserID, hashed); err != nil {
		return err
	}
	s.clearAccountFail(ctx, u.Username)

	// 改密即全端下线：撤销全部 refresh + 全部 access 会话。
	// 失败不回滚改密——口令已改是既成事实，此处仅记录并向上返回，
	// 避免用户以为改密失败而重试导致状态混乱。
	if err := s.revokeAllSessions(ctx, claims.UserID, keepCurrentRefresh); err != nil {
		return fmt.Errorf("口令已修改，但强制下线部分失败（用户需重新登录）: %w", err)
	}
	return nil
}

// revokeAllSessions 强制该用户全端下线。
func (s *Profile) revokeAllSessions(ctx context.Context, userID int64, keepRefresh string) error {
	if s.authM == nil {
		return nil
	}
	return s.authM.RevokeUserSessionsExcept(ctx, userID, keepRefresh)
}

// clearAccountFail 清空账号登录失败计数（改密成功后调用）。
func (s *Profile) clearAccountFail(ctx context.Context, username string) {
	if s.authLimiter != nil {
		s.authLimiter.Success(ctx, username)
	}
}

// SetDisabled 封禁 / 解封账号，并强制其全端下线。
//
// **封禁必须强制下线**：只标记 disabled 不够——用户已签发的 access 在
// 有效期内仍能通过守卫（本服务每请求校验 Redis 会话态，故只要撤销会话，
// 下一次请求即401）。故封禁成功后立即撤销该用户全部 refresh + 会话。
//
// 解封时同样撤销会话：解除封禁不应自动恢复攻击者已获取的凭据可用性。
//
// ⚠️ 本方法**不应由 C 端自助调用**（否则用户能自己解封）。
// 故未暴露 HTTP 端点，留待管理端接入。
func (s *Profile) SetDisabled(ctx context.Context, userID int64, disabled bool) error {
	if err := dao.SetDisabled(ctx, userID, disabled); err != nil {
		return err
	}
	// 无论封禁还是解封，都强制重新登录。
	return s.revokeAllSessions(ctx, userID, "")
}

// SetDisabledByUsername 按用户名封禁（管理端常用，管理员已知用户名）。
func (s *Profile) SetDisabledByUsername(ctx context.Context, username string, disabled bool) error {
	u, err := dao.GetUserByUsername(ctx, username)
	if err != nil {
		return err
	}
	return s.SetDisabled(ctx, u.ID, disabled)
}
