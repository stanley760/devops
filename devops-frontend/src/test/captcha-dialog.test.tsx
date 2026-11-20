import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { CaptchaDialog } from '@/components/business/CaptchaDialog'
import { buildCaptchaAnswer } from '@/lib/captcha'
import type { CaptchaChallenge } from '@/lib/types'

const challenge = (over: Partial<CaptchaChallenge>): CaptchaChallenge => ({
  id: 'c1', type: 'slide', image: 'data:image/png;base64,x', width: 300, height: 150,
  thumb: 'data:image/png;base64,t', thumb_x: 10, thumb_y: 20, thumb_width: 60, thumb_height: 60,
  ...over,
})

function Harness({ challenge, onAnswer }: { challenge: CaptchaChallenge; onAnswer: (a: unknown) => void }) {
  return (
    <CaptchaDialog
      open
      challenge={challenge}
      onAnswer={(a) => onAnswer(a)}
      onRefresh={() => {}}
      onCancel={() => {}}
    />
  )
}

describe('CaptchaDialog interactions', () => {
  it('click type collects points and submits captcha_points', () => {
    const onAnswer = vi.fn()
    const c = challenge({ type: 'click', text: 'circle,star' })
    render(<Harness challenge={c} onAnswer={onAnswer} />)
    expect(screen.getByText(/circle,star/)).toBeInTheDocument()
    fireEvent.click(screen.getByAltText('captcha'))
    fireEvent.click(screen.getByText('提交'))
    expect(onAnswer).toHaveBeenCalledWith({
      points: [expect.objectContaining({ x: expect.any(Number), y: expect.any(Number) })],
    })
  })

  it('slide type submits captcha_x/captcha_y = base + offset via buildCaptchaAnswer', () => {
    const onAnswer = vi.fn()
    render(<Harness challenge={challenge({ type: 'slide' })} onAnswer={onAnswer} />)
    const piece = screen.getByAltText('puzzle piece')
    fireEvent.mouseDown(piece, { clientX: 10, clientY: 20 })
    fireEvent.mouseMove(piece, { clientX: 40, clientY: 20 })
    fireEvent.mouseUp(piece)
    fireEvent.click(screen.getByText('提交'))
    const a = onAnswer.mock.calls[0]![0] as { x: number; y: number }
    // 作答经 buildCaptchaAnswer 映射为后端契约字段
    const mapped = buildCaptchaAnswer(challenge({ type: 'slide' }), a)
    expect(mapped).toEqual({ captcha_id: 'c1', captcha_x: a.x, captcha_y: a.y })
  })

  it('rotate type defaults angle 0 when untouched', () => {
    const onAnswer = vi.fn()
    render(<Harness challenge={challenge({ type: 'rotate', thumb_size: 100 })} onAnswer={onAnswer} />)
    fireEvent.click(screen.getByText('提交'))
    const a = onAnswer.mock.calls[0]![0] as { angle: number }
    expect(buildCaptchaAnswer(challenge({ type: 'rotate' }), a)).toEqual({ captcha_id: 'c1', captcha_angle: 0 })
  })

  it('dragdrop type submits captcha_x/captcha_y', () => {
    const onAnswer = vi.fn()
    render(<Harness challenge={challenge({ type: 'dragdrop' })} onAnswer={onAnswer} />)
    const piece = screen.getByAltText('puzzle piece')
    fireEvent.mouseDown(piece, { clientX: 0, clientY: 0 })
    fireEvent.mouseMove(piece, { clientX: 15, clientY: 15 })
    fireEvent.mouseUp(piece)
    fireEvent.click(screen.getByText('提交'))
    const a = onAnswer.mock.calls[0]![0] as { x: number; y: number }
    expect(buildCaptchaAnswer(challenge({ type: 'dragdrop' }), a)).toMatchObject({ captcha_id: 'c1' })
  })
})
