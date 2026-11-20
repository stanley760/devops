import { describe, it, expect } from 'vitest'
import { captchaEnabled, buildCaptchaAnswer } from '@/lib/captcha'
import type { CaptchaChallenge } from '@/lib/types'

const base = (over: Partial<CaptchaChallenge>): CaptchaChallenge => ({
  id: 'c1', type: 'slide', image: 'img', width: 300, height: 150, ...over,
})

describe('captchaEnabled', () => {
  it('enabled=false skips', () => expect(captchaEnabled(base({ enabled: false }))).toBe(false))
  it('missing enabled treated as on', () => expect(captchaEnabled(base({}))).toBe(true))
  it('null challenge skipped', () => expect(captchaEnabled(null)).toBe(false))
})

describe('buildCaptchaAnswer', () => {
  it('click maps captcha_points', () => {
    expect(buildCaptchaAnswer(base({ type: 'click' }), { points: [{ x: 1, y: 2 }] }))
      .toEqual({ captcha_id: 'c1', captcha_points: [{ x: 1, y: 2 }] })
  })
  it('slide maps captcha_x/captcha_y', () => {
    expect(buildCaptchaAnswer(base({ type: 'slide' }), { x: 10, y: 20 }))
      .toEqual({ captcha_id: 'c1', captcha_x: 10, captcha_y: 20 })
  })
  it('dragdrop maps captcha_x/captcha_y', () => {
    expect(buildCaptchaAnswer(base({ type: 'dragdrop' }), { x: 5, y: 6 }))
      .toEqual({ captcha_id: 'c1', captcha_x: 5, captcha_y: 6 })
  })
  it('rotate maps captcha_angle (0 when no drag)', () => {
    expect(buildCaptchaAnswer(base({ type: 'rotate' }), {})).toEqual({ captcha_id: 'c1', captcha_angle: 0 })
    expect(buildCaptchaAnswer(base({ type: 'rotate' }), { angle: 90 })).toEqual({ captcha_id: 'c1', captcha_angle: 90 })
  })
})
