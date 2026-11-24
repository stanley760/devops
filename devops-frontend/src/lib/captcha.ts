import type { CaptchaAnswer, CaptchaChallenge } from '@/lib/types'

export function captchaEnabled(c: CaptchaChallenge | null | undefined): boolean {
  if (!c) return false
  return c.enabled !== false
}

export function buildCaptchaAnswer(c: CaptchaChallenge, a: CaptchaAnswer): Record<string, unknown> {
  switch (c.type) {
    case 'click':
      return { captcha_id: c.id, captcha_points: a.points ?? [] }
    case 'slide':
    case 'dragdrop':
      return { captcha_id: c.id, captcha_x: a.x ?? 0, captcha_y: a.y ?? 0 }
    case 'rotate':
      return { captcha_id: c.id, captcha_angle: a.angle ?? 0 }
  }
}
