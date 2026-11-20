import type { CaptchaAnswer, CaptchaChallenge } from '@/lib/types'

export function captchaEnabled(c: CaptchaChallenge | null | undefined): boolean {
  if (!c) return false
  return c.enabled !== false
}

export function buildCaptchaAnswer(c: CaptchaChallenge, a: CaptchaAnswer): Record<string, unknown> {
  const base: Record<string, unknown> = { captcha_id: c.id }
  switch (c.type) {
    case 'click':
      base.captcha_points = a.points ?? []
      break
    case 'slide':
    case 'dragdrop':
      base.captcha_x = a.x ?? 0
      base.captcha_y = a.y ?? 0
      break
    case 'rotate':
      base.captcha_angle = a.angle ?? 0
      break
  }
  return base
}
