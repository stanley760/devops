/**
 * C 端门户子页面：登录 / 注册 / 个人资料 / 产品文档占位。
 * 与 hero 同风格（黑底 + Poppins + 玻璃拟态卡片）。
 */
import { portalApi, portalSession, PortalApiError, type PublicProfile } from './api'
import { CaptchaWidget } from './captcha'

function navHtml(): string {
  const authed = portalSession.loggedIn
  return `
  <header class="nav">
    <a href="#/" class="logo"><i class="logo-mark"></i><b>Design</b><span>World</span></a>
    <ul class="nav-links">
      <li><a href="#/">Home</a></li>
      <li class="hide-sm"><a href="#/docs">产品文档</a></li>
      ${authed
        ? '<li><a href="#/profile">个人中心</a></li><li><a href="javascript:void 0" data-logout>退出</a></li>'
        : '<li><a href="#/login">登录</a></li><li><a href="#/register" class="solid">注册</a></li>'}
    </ul>
  </header>`
}

function bindLogout(root: HTMLElement): void {
  root.querySelector('[data-logout]')?.addEventListener('click', async () => {
    try { await portalApi.logout() } catch { /* 忽略 */ }
    portalSession.clear()
    location.hash = '#/'
  })
}

function errMsg(e: unknown): string {
  return e instanceof PortalApiError ? e.message : '网络异常，请稍后重试'
}

/**
 * 挂载行为式验证码到表单。
 * 后端开启验证码时（sys.captcha.enabled，默认开启）登录/注册必须携带作答，
 * 否则一律拒绝——所以这两个表单必须带验证码。
 */
function mountCaptcha(form: HTMLFormElement): CaptchaWidget {
  const host = form.querySelector('[data-captcha]') as HTMLDivElement
  const widget = new CaptchaWidget({
    onSolved: () => { /* 提交时统一取值 */ },
    onRefresh: () => portalApi.captcha(),
  })
  host.appendChild(widget.el)
  widget.bind()
  void widget.load()
  return widget
}

// ---------------- 登录 ----------------
export function renderLogin(root: HTMLElement): void {
  if (portalSession.loggedIn) { location.hash = '#/profile'; return }
  root.innerHTML = `
  <div class="auth-page">
    ${navHtml()}
    <div class="auth-body">
      <form class="auth-card" id="form">
        <h2>Welcome Back</h2>
        <p class="sub">登录以进入你的 Design World 工作台</p>
        <div class="field"><label>用户名</label><input name="username" autocomplete="username" required minlength="3" maxlength="32" placeholder="alice"></div>
        <div class="field"><label>密码</label><input name="password" type="password" autocomplete="current-password" required minlength="8" placeholder="至少 8 位"></div>
        <div class="field"><label>验证码</label><div data-captcha></div></div>
        <div class="form-error" id="err"></div>
        <button class="btn-primary" type="submit">Sign In</button>
        <p class="auth-switch">还没有账号？<a href="#/register">立即注册</a></p>
      </form>
    </div>
  </div>`
  bindLogout(root)
  const form = root.querySelector<HTMLFormElement>('#form')!
  const err = root.querySelector<HTMLElement>('#err')!
  const captcha = mountCaptcha(form)
  form.addEventListener('submit', async (e) => {
    e.preventDefault()
    err.textContent = ''
    const btn = form.querySelector<HTMLButtonElement>('button')!
    btn.disabled = true
    const fd = new FormData(form)
    try {
      const pair = await portalApi.login({
        username: String(fd.get('username')).trim(),
        password: String(fd.get('password')),
        captcha: captcha.answer() ?? undefined,
      })
      portalSession.save(pair)
      location.hash = '#/profile'
    } catch (ex) {
      err.textContent = errMsg(ex)
      btn.disabled = false
      // 挑战是一次性的，失败后必须换一张
      void captcha.load()
    }
  })
}

// ---------------- 注册 ----------------
export function renderRegister(root: HTMLElement): void {
  if (portalSession.loggedIn) { location.hash = '#/profile'; return }
  root.innerHTML = `
  <div class="auth-page">
    ${navHtml()}
    <div class="auth-body">
      <form class="auth-card" id="form">
        <h2>Create Account</h2>
        <p class="sub">注册即登录，开启你的产品之旅</p>
        <div class="field"><label>用户名 *</label><input name="username" autocomplete="username" required minlength="3" maxlength="32" placeholder="3-32 位登录名"></div>
        <div class="field"><label>昵称</label><input name="nickname" maxlength="32" placeholder="默认同用户名"></div>
        <div class="field"><label>邮箱</label><input name="email" type="email" placeholder="alice@example.com"></div>
        <div class="field"><label>密码 *</label><input name="password" type="password" autocomplete="new-password" required minlength="8" placeholder="至少 8 位"></div>
        <div class="field"><label>确认密码 *</label><input name="confirm" type="password" autocomplete="new-password" required minlength="8"></div>
        <div class="field"><label>验证码</label><div data-captcha></div></div>
        <div class="form-error" id="err"></div>
        <button class="btn-primary" type="submit">Sign Up</button>
        <p class="auth-switch">已有账号？<a href="#/login">直接登录</a></p>
      </form>
    </div>
  </div>`
  bindLogout(root)
  const form = root.querySelector<HTMLFormElement>('#form')!
  const err = root.querySelector<HTMLElement>('#err')!
  const captcha = mountCaptcha(form)
  form.addEventListener('submit', async (e) => {
    e.preventDefault()
    err.textContent = ''
    const fd = new FormData(form)
    const password = String(fd.get('password'))
    if (password !== String(fd.get('confirm'))) {
      err.textContent = '两次输入的密码不一致'
      return
    }
    const btn = form.querySelector<HTMLButtonElement>('button')!
    btn.disabled = true
    try {
      const pair = await portalApi.register({
        username: String(fd.get('username')).trim(),
        password,
        nickname: String(fd.get('nickname')).trim() || undefined,
        email: String(fd.get('email')).trim() || undefined,
        captcha: captcha.answer() ?? undefined,
      })
      portalSession.save(pair)
      location.hash = '#/profile'
    } catch (ex) {
      err.textContent = errMsg(ex)
      btn.disabled = false
      void captcha.load()
    }
  })
}

// ---------------- 个人资料 ----------------
export async function renderProfile(root: HTMLElement): Promise<void> {
  if (!portalSession.loggedIn) { location.hash = '#/login'; return }
  root.innerHTML = `
  <div class="profile-page">
    ${navHtml()}
    <div class="profile-wrap"><p style="opacity:.6;letter-spacing:.2em;text-transform:uppercase;font-size:12px">Loading profile</p></div>
  </div>`
  bindLogout(root)

  let profile: PublicProfile
  try {
    profile = await portalApi.profile()
  } catch (ex) {
    if (ex instanceof PortalApiError && ex.code === 401) { location.hash = '#/login'; return }
    root.querySelector('.profile-wrap')!.innerHTML = `<p class="form-error">${errMsg(ex)}</p>`
    return
  }

  const genderText = ['未知', '男', '女'][profile.gender] ?? '未知'
  const created = profile.created_at ? new Date(profile.created_at).toLocaleString('zh-CN') : '-'
  const initial = (profile.nickname || profile.username || 'U').slice(0, 1).toUpperCase()

  root.innerHTML = `
  <div class="profile-page">
    ${navHtml()}
    <div class="profile-wrap">
      <div class="profile-head">
        <div class="profile-avatar">${profile.avatar ? `<img src="${profile.avatar}" alt="avatar">` : initial}</div>
        <div>
          <h2>${profile.nickname || profile.username}</h2>
          <p>@${profile.username} · UID ${profile.id}</p>
        </div>
      </div>
      <div class="profile-meta">
        <div class="meta-item"><div class="k">邮箱</div><div class="v">${profile.email || '未绑定'}</div></div>
        <div class="meta-item"><div class="k">手机号</div><div class="v">${profile.mobile || '未绑定'}</div></div>
        <div class="meta-item"><div class="k">性别</div><div class="v">${genderText}</div></div>
        <div class="meta-item"><div class="k">注册时间</div><div class="v">${created}</div></div>
      </div>
      <div class="profile-grid">
        <form class="panel" id="profile-form">
          <h3>编辑资料</h3>
          <div class="field"><label>昵称</label><input name="nickname" maxlength="32" value="${profile.nickname ?? ''}"></div>
          <div class="field"><label>邮箱</label><input name="email" type="email" value="${profile.email ?? ''}"></div>
          <div class="field"><label>手机号</label><input name="mobile" value="${profile.mobile ?? ''}"></div>
          <div class="field"><label>性别</label>
            <select name="gender">
              <option value="0" ${profile.gender === 0 ? 'selected' : ''}>未知</option>
              <option value="1" ${profile.gender === 1 ? 'selected' : ''}>男</option>
              <option value="2" ${profile.gender === 2 ? 'selected' : ''}>女</option>
            </select>
          </div>
          <div class="form-ok" id="ok1"></div>
          <div class="form-error" id="err1"></div>
          <button class="btn-primary" type="submit">保存修改</button>
        </form>
        <form class="panel" id="pwd-form">
          <h3>修改密码</h3>
          <div class="field"><label>当前密码</label><input name="old_password" type="password" autocomplete="current-password" required></div>
          <div class="field"><label>新密码</label><input name="new_password" type="password" autocomplete="new-password" required minlength="8" placeholder="至少 8 位"></div>
          <div class="field"><label>确认新密码</label><input name="confirm" type="password" autocomplete="new-password" required minlength="8"></div>
          <div class="form-ok" id="ok2"></div>
          <div class="form-error" id="err2"></div>
          <button class="btn-ghost" type="submit">更新密码</button>
          <p class="auth-switch" style="margin-top:24px"><a href="javascript:void 0" data-logout>退出登录</a></p>
        </form>
      </div>
    </div>
  </div>`
  bindLogout(root)

  const pform = root.querySelector<HTMLFormElement>('#profile-form')!
  pform.addEventListener('submit', async (e) => {
    e.preventDefault()
    const ok = root.querySelector<HTMLElement>('#ok1')!
    const err = root.querySelector<HTMLElement>('#err1')!
    ok.textContent = ''; err.textContent = ''
    const fd = new FormData(pform)
    try {
      await portalApi.updateProfile({
        nickname: String(fd.get('nickname')).trim() || undefined,
        email: String(fd.get('email')).trim() || undefined,
        mobile: String(fd.get('mobile')).trim() || undefined,
        gender: Number(fd.get('gender')),
      })
      ok.textContent = '已保存'
      setTimeout(() => void renderProfile(root), 600)
    } catch (ex) {
      err.textContent = errMsg(ex)
    }
  })

  const wform = root.querySelector<HTMLFormElement>('#pwd-form')!
  wform.addEventListener('submit', async (e) => {
    e.preventDefault()
    const ok = root.querySelector<HTMLElement>('#ok2')!
    const err = root.querySelector<HTMLElement>('#err2')!
    ok.textContent = ''; err.textContent = ''
    const fd = new FormData(wform)
    const np = String(fd.get('new_password'))
    if (np !== String(fd.get('confirm'))) { err.textContent = '两次输入的新密码不一致'; return }
    try {
      await portalApi.changePassword({
        old_password: String(fd.get('old_password')),
        new_password: np,
        keep_current_device: true,
      })
      ok.textContent = '密码已更新'
      wform.reset()
    } catch (ex) {
      err.textContent = errMsg(ex)
    }
  })
}

// ---------------- 产品文档（占位介绍页） ----------------
export function renderDocs(root: HTMLElement): void {
  root.innerHTML = `
  <div class="profile-page">
    ${navHtml()}
    <div class="profile-wrap">
      <div class="profile-head">
        <div>
          <h2>产品文档</h2>
          <p>DevOps 一站式运维管理平台 · 产品能力总览</p>
        </div>
      </div>
      <div class="profile-grid">
        <div class="panel"><h3>平台简介</h3>
          <p style="font-weight:300;line-height:1.9;color:rgba(255,255,255,.75);font-size:14px">
            Design World 是本 DevOps 平台的 C 端门户。平台由微服务架构支撑：
            用户中心（devops-user）提供注册、登录、令牌刷新与个人资料管理；
            权限系统（devops-system）提供租户、角色与动态菜单；
            基础设施服务（devops-infra）提供监控与资源编排能力。
          </p>
        </div>
        <div class="panel"><h3>快速开始</h3>
          <p style="font-weight:300;line-height:1.9;color:rgba(255,255,255,.75);font-size:14px">
            1. 注册账号并登录；<br>
            2. 在「个人中心」完善资料；<br>
            3. 联系管理员开通租户权限后，即可进入后台工作台。
          </p>
          <p style="margin-top:20px"><a class="cta" href="#/register">立即注册</a></p>
        </div>
      </div>
    </div>
  </div>`
  bindLogout(root)
}
