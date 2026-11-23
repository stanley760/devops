import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Configs from '@/views/system/Configs'
import { adminSystem } from '@/api/admin/system'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import { invalidateOptions } from '@/lib/options'
import type { PageResult, SystemConfig } from '@/lib/types'

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listConfigs: vi.fn(),
    createConfig: vi.fn().mockResolvedValue(undefined),
    updateConfig: vi.fn().mockResolvedValue(undefined),
    deleteConfig: vi.fn().mockResolvedValue(undefined),
    listDictItems: vi.fn().mockResolvedValue([
      { id: '1', dictType: 'sys_captcha_type', label: '滑动', value: 'slide', sort: 1 },
      { id: '2', dictType: 'sys_captcha_type', label: '旋转', value: 'rotate', sort: 2 },
    ]),
  },
}))

const config = (over: Partial<SystemConfig>): SystemConfig => ({
  id: '1', configKey: 'sys.account.registerUser', configName: '是否允许自注册',
  configValue: 'true', type: 1, visible: true, ...over,
})

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  invalidateOptions()
  vi.mocked(adminSystem.listConfigs).mockResolvedValue({
    list: [
      config({}),
      config({ id: '2', configKey: 'sys.captcha.type', configName: '验证码类型', configValue: 'slide', type: 1 }),
      config({ id: '3', configKey: 'custom.key', configName: '自定义', configValue: 'v1', type: 2 }),
    ],
    total: 3, page_num: 1, page_size: 10, pages: 1,
  } as PageResult<SystemConfig>)
})

function renderConfigs() {
  return render(
    <MemoryRouter>
      <PermissionsProvider permissions={['system:config:create', 'system:config:update', 'system:config:delete']}>
        <ConfirmProvider>
          <Configs />
        </ConfirmProvider>
      </PermissionsProvider>
    </MemoryRouter>,
  )
}

describe('Configs page', () => {
  it('renders rows with builtin badge', async () => {
    renderConfigs()
    await screen.findByText('sys.account.registerUser')
    expect(screen.getAllByText('内置')).toHaveLength(2)
  })

  it('builtin rows cannot be deleted (no delete button)', async () => {
    renderConfigs()
    await screen.findByText('sys.account.registerUser')
    // type=1 两行无删除按钮；type=2 一行有
    expect(screen.getAllByText('删除')).toHaveLength(1)
  })

  it('edits builtin switch key and sends snake_case', async () => {
    renderConfigs()
    await screen.findByText('sys.account.registerUser')
    fireEvent.click(screen.getAllByText('编辑')[0]!)
    fireEvent.click(screen.getByRole('switch', { name: '参数值' }))
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(adminSystem.updateConfig).toHaveBeenCalled())
    const [id, body] = vi.mocked(adminSystem.updateConfig).mock.calls[0]!
    expect(id).toBe('1')
    expect(body).toMatchObject({ config_key: 'sys.account.registerUser', config_value: 'false' })
    expect(Object.keys(body)).not.toContain('configKey')
  })

  it('captcha type renders dict-driven select', async () => {
    renderConfigs()
    await screen.findByText('sys.captcha.type')
    fireEvent.click(screen.getAllByText('编辑')[1]!)
    const select = await screen.findByLabelText('参数值')
    expect(select.tagName).toBe('SELECT')
    expect(screen.getByText('滑动')).toBeInTheDocument()
  })
})
