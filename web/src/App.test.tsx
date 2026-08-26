import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'

const settingsBody = (overrides: Record<string, unknown> = {}) =>
  JSON.stringify({
    listen_addr: '127.0.0.1:8041',
    database_path: './data/cursor-tab-server.db',
    proxy_rate_per_minute: 120,
    admin_rate_per_minute: 30,
    log_retention_days: 30,
    captcha_rate_per_minute: 20,
    login_rate_per_minute: 10,
    cursor_token_set: true,
    cursor_token_masked: 'curs••••3456',
    ...overrides,
  })

const captchaResponse = () => new Response(JSON.stringify({
  captcha_id: 'captcha-id',
  image: 'data:image/png;base64,cG5n',
  expires_at: '2026-08-26T06:00:00Z',
}), { status: 200, headers: { 'Content-Type': 'application/json' } })

function stubAuthorizedAPIs() {
  vi.stubGlobal('fetch', vi.fn((path: string, options?: RequestInit) => {
    if (path === '/admin/session' && options?.method === 'DELETE') return Promise.resolve(new Response(null, { status: 204 }))
    if (path === '/admin/session') return Promise.resolve(new Response(null, { status: 204 }))
    if (path === '/admin/captcha') return Promise.resolve(captchaResponse())
    if (path === '/admin/settings' && options?.method === 'PUT') return Promise.resolve(new Response(settingsBody({ proxy_rate_per_minute: 240 }), { status: 200 }))
    if (path === '/admin/settings') return Promise.resolve(new Response(settingsBody(), { status: 200 }))
    if (path === '/admin/dashboard') return Promise.resolve(new Response(JSON.stringify({
      requests_24h: 1284, errors_24h: 3, average_latency_ms: 148, success_rate: 99.77,
      status_distribution: [{ status_code: 200, count: 1281 }, { status_code: 502, count: 3 }],
      server: { started_at: '2026-08-26T06:00:00Z', listen_addr: '127.0.0.1:8041' },
    }), { status: 200 }))
    if (path.startsWith('/admin/audit-logs')) return Promise.resolve(new Response(JSON.stringify({ items: [], total: 0, limit: 50, offset: 0 }), { status: 200 }))
    if (path === '/admin/api-keys' && options?.method === 'POST') return Promise.resolve(new Response(JSON.stringify({ secret: 'cts_created-key-value' }), { status: 201 }))
    if (path.startsWith('/admin/api-keys')) return Promise.resolve(new Response(JSON.stringify({ items: [], total: 0, limit: 50, offset: 0 }), { status: 200 }))
    if (path === '/admin/status') return Promise.resolve(new Response(JSON.stringify({ database: 'ok', started_at: '2026-08-26T06:00:00Z', proxy_rate_per_minute: 120, admin_rate_per_minute: 30, log_retention_days: 30 }), { status: 200 }))
    return Promise.resolve(new Response(null, { status: 404 }))
  }))
}

describe('enterprise SaaS administration console', () => {
  afterEach(() => cleanup())

  beforeEach(() => {
    window.history.replaceState({}, '', '/login')
    window.localStorage.clear()
    window.sessionStorage.clear()
    vi.stubGlobal('fetch', vi.fn((path: string) => {
      if (path === '/admin/session') return Promise.resolve(new Response(null, { status: 401 }))
      if (path === '/admin/captcha') return Promise.resolve(captchaResponse())
      return Promise.resolve(new Response(null, { status: 404 }))
    }))
  })

  it('loads a captcha and sends credentials only in the login request', async () => {
    render(<App />)
    expect((await screen.findByRole('img', { name: '图形验证码' })).getAttribute('src')).toBe('data:image/png;base64,cG5n')

    fireEvent.change(screen.getByLabelText('用户名'), { target: { value: 'admin' } })
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'correct-password' } })
    fireEvent.change(screen.getByLabelText('验证码'), { target: { value: 'ABC123' } })
    fireEvent.submit(screen.getByRole('button', { name: '登录' }).closest('form')!)

    await waitFor(() => expect(fetch).toHaveBeenCalledWith('/admin/session', expect.objectContaining({ method: 'POST' })))
    const [, options] = vi.mocked(fetch).mock.calls.find(([path, options]) => path === '/admin/session' && options?.method === 'POST')!
    expect(options?.body).toBe(JSON.stringify({ username: 'admin', password: 'correct-password', captcha_id: 'captcha-id', captcha_answer: 'ABC123' }))
    expect(window.localStorage.length).toBe(0)
    expect(window.sessionStorage.length).toBe(0)
  })

  it('refreshes the captcha and clears the answer after authentication fails', async () => {
    let captchaCount = 0
    vi.stubGlobal('fetch', vi.fn((path: string) => {
      if (path === '/admin/session') return Promise.resolve(new Response(JSON.stringify({ error: 'invalid_credentials' }), { status: 401 }))
      if (path === '/admin/captcha') {
        captchaCount++
        return Promise.resolve(captchaResponse())
      }
      return Promise.resolve(new Response(null, { status: 404 }))
    }))
    render(<App />)
    await screen.findByRole('img', { name: '图形验证码' })
    fireEvent.change(screen.getByLabelText('验证码'), { target: { value: 'ABC123' } })
    fireEvent.submit(screen.getByRole('button', { name: '登录' }).closest('form')!)

    await waitFor(() => expect(screen.getByText('用户名、密码或验证码无效')).toBeTruthy())
    expect((screen.getByLabelText('验证码') as HTMLInputElement).value).toBe('')
    expect(captchaCount).toBe(2)
  })

  it('renders an operational dashboard for authorized sessions', async () => {
    stubAuthorizedAPIs()
    window.history.replaceState({}, '', '/')
    render(<App />)

    expect(await screen.findByText('运营仪表盘')).toBeTruthy()
    expect(await screen.findByText('1,284')).toBeTruthy()
    expect(screen.getByText('99.8%')).toBeTruthy()
    expect(screen.getByText('状态分布')).toBeTruthy()
    expect(screen.getByRole('link', { name: 'API 密钥' })).toBeTruthy()
  })

  it('shows current system configuration with editable rate limits', async () => {
    stubAuthorizedAPIs()
    window.history.replaceState({}, '', '/settings')
    render(<App />)

    expect(await screen.findByRole('heading', { name: '系统配置' })).toBeTruthy()
    expect((await screen.findByLabelText('监听地址') as HTMLInputElement).value).toBe('127.0.0.1:8041')
    expect((screen.getByLabelText('数据库路径') as HTMLInputElement).value).toBe('./data/cursor-tab-server.db')
    expect((screen.getByLabelText('验证码获取限流') as HTMLInputElement).value).toBe('20')
    expect((screen.getByLabelText('代理调用限流') as HTMLInputElement).value).toBe('120')
  })

  it('saves edited settings online and confirms the change', async () => {
    stubAuthorizedAPIs()
    window.history.replaceState({}, '', '/settings')
    render(<App />)

    const input = (await screen.findByLabelText('代理调用限流')) as HTMLInputElement
    expect(screen.getByRole('button', { name: '保存更改' })).toHaveProperty('disabled', true)

    fireEvent.change(input, { target: { value: '240' } })
    const save = screen.getByRole('button', { name: '保存更改' })
    expect(save).toHaveProperty('disabled', false)
    fireEvent.click(save)

    await waitFor(() => expect(fetch).toHaveBeenCalledWith('/admin/settings', expect.objectContaining({ method: 'PUT' })))
    const [, options] = vi.mocked(fetch).mock.calls.find(([path, options]) => path === '/admin/settings' && options?.method === 'PUT')!
    expect(JSON.parse(String(options?.body))).toMatchObject({ proxy_rate_per_minute: 240, admin_rate_per_minute: 30 })
    expect(await screen.findByText('配置已保存并立即生效')).toBeTruthy()
    expect((screen.getByLabelText('代理调用限流') as HTMLInputElement).value).toBe('240')
  })

  it('keeps the save button disabled for out-of-range values', async () => {
    stubAuthorizedAPIs()
    window.history.replaceState({}, '', '/settings')
    render(<App />)

    const input = (await screen.findByLabelText('登录尝试限流')) as HTMLInputElement
    fireEvent.change(input, { target: { value: '5000' } })
    expect(screen.getByRole('button', { name: '保存更改' })).toHaveProperty('disabled', true)
  })

  it('creates a key and shows a Cursor TAB service address', async () => {
    stubAuthorizedAPIs()
    window.history.replaceState({}, '', '/api-keys')
    render(<App />)

    fireEvent.click(await screen.findByRole('button', { name: '创建密钥' }))
    fireEvent.change(screen.getByLabelText('名称'), { target: { value: 'Cursor desktop' } })
    fireEvent.click(screen.getByRole('button', { name: '确认创建' }))

    expect(await screen.findByText('Cursor TAB 服务地址')).toBeTruthy()
    expect(screen.getByText(`${window.location.origin}/key=cts_created-key-value`)).toBeTruthy()
  })

  it('updates the cursor token online without exposing the full value', async () => {
    stubAuthorizedAPIs()
    window.history.replaceState({}, '', '/settings')
    render(<App />)

    expect(((await screen.findByLabelText('当前令牌')) as HTMLInputElement).value).toBe('curs••••3456')
    const input = screen.getByLabelText('新令牌') as HTMLInputElement
    expect(screen.getByRole('button', { name: '更新 Token' })).toHaveProperty('disabled', true)

    fireEvent.change(input, { target: { value: '  new-token-123456  ' } })
    fireEvent.click(screen.getByRole('button', { name: '更新 Token' }))

    await waitFor(() => expect(fetch).toHaveBeenCalledWith('/admin/settings', expect.objectContaining({ method: 'PUT' })))
    const calls = vi.mocked(fetch).mock.calls.filter(([path, options]) => path === '/admin/settings' && options?.method === 'PUT')
    const [, options] = calls[calls.length - 1]
    expect(JSON.parse(String(options?.body))).toEqual({ cursor_token: 'new-token-123456' })
    expect(await screen.findByText('Cursor Token 已更新并立即生效')).toBeTruthy()
    expect(input.value).toBe('')
  })
})
