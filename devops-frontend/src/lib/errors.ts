export function mapLoginError(raw: string): string {
  const msg = raw.toLowerCase()
  if (msg.includes('incorrect username or password')) return '账号或密码错误'
  if (msg.includes('user does not exist')) return '账号不存在或已被删除'
  if (msg.includes('tenant not found')) return '租户不存在，请重新选择'
  if (msg.includes('tenant is disabled')) return '租户已被禁用，请联系管理员'
  if (msg.includes('too many') || msg.includes('rate limit')) return '操作过于频繁，请稍后再试'
  return raw
}
