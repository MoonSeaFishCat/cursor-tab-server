import { useEffect, useMemo, useState } from 'react'
import { KeyRound, RotateCcw, Save } from 'lucide-react'
import { api, ApiError } from '../lib/api'
import type { SystemSettings } from '../lib/types'
import { Button } from '../components/ui/button'
import { PageHeader, Panel, Toast } from '../components/kit'

type NumericKey = 'proxy_rate_per_minute' | 'admin_rate_per_minute' | 'captcha_rate_per_minute' | 'login_rate_per_minute' | 'log_retention_days'

type FieldSpec = {
  key: NumericKey
  label: string
  hint: string
  min: number
  max: number
  unit: string
}

const rateFields: FieldSpec[] = [
  { key: 'proxy_rate_per_minute', label: '代理调用限流', hint: '每个密钥 + IP 每分钟允许的最大请求数', min: 1, max: 100000, unit: '次/分钟' },
  { key: 'admin_rate_per_minute', label: '管理接口限流', hint: '每个来源 IP 每分钟允许的管理 API 请求数', min: 1, max: 100000, unit: '次/分钟' },
  { key: 'captcha_rate_per_minute', label: '验证码获取限流', hint: '每个来源 IP 每分钟可获取的图形验证码数量', min: 1, max: 100000, unit: '次/分钟' },
  { key: 'login_rate_per_minute', label: '登录尝试限流', hint: '每个来源 IP 每分钟允许的登录尝试次数', min: 1, max: 1000, unit: '次/分钟' },
]

const dataFields: FieldSpec[] = [
  { key: 'log_retention_days', label: '日志保留天数', hint: '审计日志超过该天数后会被自动清理', min: 1, max: 3650, unit: '天' },
]

const errorText: Record<string, string> = {
  invalid_proxy_rate_per_minute: '代理调用限流需在 1–100000 之间',
  invalid_admin_rate_per_minute: '管理接口限流需在 1–100000 之间',
  invalid_captcha_rate_per_minute: '验证码获取限流需在 1–100000 之间',
  invalid_login_rate_per_minute: '登录尝试限流需在 1–1000 之间',
  invalid_log_retention_days: '日志保留天数需在 1–3650 之间',
  invalid_cursor_token: 'Cursor Token 长度需在 10–4096 个字符之间',
}

function NumericField({ spec, value, onChange }: { spec: FieldSpec; value: string; onChange: (value: string) => void }) {
  return (
    <label className="field">
      <span className="field-label">{spec.label}</span>
      <span className="input-suffix">
        <input
          type="number"
          inputMode="numeric"
          aria-label={spec.label}
          min={spec.min}
          max={spec.max}
          value={value}
          onChange={event => onChange(event.target.value)}
        />
        <span className="suffix">{spec.unit}</span>
      </span>
      <span className="field-hint">
        {spec.hint}（{spec.min}–{spec.max}）
      </span>
    </label>
  )
}

export function SettingsPage() {
  const [settings, setSettings] = useState<SystemSettings>()
  const [form, setForm] = useState<Record<NumericKey, string>>({
    proxy_rate_per_minute: '',
    admin_rate_per_minute: '',
    captcha_rate_per_minute: '',
    login_rate_per_minute: '',
    log_retention_days: '',
  })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)
  const [tokenInput, setTokenInput] = useState('')
  const [tokenSaving, setTokenSaving] = useState(false)
  const [tokenError, setTokenError] = useState('')
  const [tokenSaved, setTokenSaved] = useState(false)

  const applySettings = (value: SystemSettings) => {
    setSettings(value)
    setForm({
      proxy_rate_per_minute: String(value.proxy_rate_per_minute),
      admin_rate_per_minute: String(value.admin_rate_per_minute),
      captcha_rate_per_minute: String(value.captcha_rate_per_minute),
      login_rate_per_minute: String(value.login_rate_per_minute),
      log_retention_days: String(value.log_retention_days),
    })
  }

  useEffect(() => {
    api<SystemSettings>('/admin/settings').then(applySettings).catch(() => setError('配置加载失败，请刷新后重试。'))
  }, [])

  const dirty = useMemo(() => {
    if (!settings) return false
    return (Object.keys(form) as NumericKey[]).some(key => form[key] !== String(settings[key]))
  }, [form, settings])

  const setValue = (key: NumericKey) => (value: string) => {
    setError('')
    setForm(current => ({ ...current, [key]: value }))
  }

  const invalid = (Object.keys(form) as NumericKey[]).some(key => {
    const spec = [...rateFields, ...dataFields].find(field => field.key === key)!
    const value = Number(form[key])
    return !Number.isInteger(value) || value < spec.min || value > spec.max
  })

  const save = async () => {
    setSaving(true)
    setError('')
    const payload: Partial<Record<NumericKey, number>> = {}
    for (const key of Object.keys(form) as NumericKey[]) {
      payload[key] = Number(form[key])
    }
    try {
      const updated = await api<SystemSettings>('/admin/settings', { method: 'PUT', body: JSON.stringify(payload) })
      applySettings(updated)
      setSaved(true)
    } catch (cause) {
      setError(cause instanceof ApiError ? errorText[cause.code] ?? '保存失败，请检查输入后重试。' : '保存失败，请检查网络后重试。')
    } finally {
      setSaving(false)
    }
  }

  const reset = () => {
    if (settings) applySettings(settings)
    setError('')
  }

  const saveToken = async () => {
    const token = tokenInput.trim()
    if (!token) return
    setTokenSaving(true)
    setTokenError('')
    try {
      const updated = await api<SystemSettings>('/admin/settings', { method: 'PUT', body: JSON.stringify({ cursor_token: token }) })
      setSettings(updated)
      setTokenInput('')
      setTokenSaved(true)
    } catch (cause) {
      setTokenError(cause instanceof ApiError ? errorText[cause.code] ?? 'Token 保存失败，请检查后重试。' : 'Token 保存失败，请检查网络后重试。')
    } finally {
      setTokenSaving(false)
    }
  }

  return (
    <>
      <PageHeader
        eyebrow="Configuration"
        title="系统配置"
        description="在线修改限流与日志保留策略，保存后立即生效并持久化到数据库。"
        actions={
          <>
            <Button variant="secondary" onClick={reset} disabled={!dirty || saving}>
              <RotateCcw size={14} />
              还原
            </Button>
            <Button onClick={() => void save()} disabled={!dirty || invalid || saving}>
              <Save size={14} />
              {saving ? '正在保存…' : '保存更改'}
            </Button>
          </>
        }
      />
      {error && <p className="notice err page-notice">{error}</p>}
      <div className="settings-grid">
        <Panel title="限流策略" subtitle="调整各入口的速率限制，保存后立即生效">
          {rateFields.map(spec => (
            <NumericField key={spec.key} spec={spec} value={form[spec.key]} onChange={setValue(spec.key)} />
          ))}
        </Panel>
        <div className="settings-side">
          <Panel title="Cursor 凭证" subtitle="代理转发上游时使用的 Cursor Token，保存后立即生效">
            <label className="field">
              <span className="field-label">当前令牌</span>
              <input aria-label="当前令牌" value={settings ? (settings.cursor_token_set ? settings.cursor_token_masked : '未设置') : ''} readOnly className="readonly mono" />
            </label>
            <label className="field">
              <span className="field-label">新令牌</span>
              <input
                type="password"
                aria-label="新令牌"
                value={tokenInput}
                onChange={event => {
                  setTokenInput(event.target.value)
                  setTokenError('')
                }}
                placeholder="粘贴新的 Cursor Token"
                autoComplete="off"
              />
              <span className="field-hint">保存后立即替换当前服务使用的 Token 并持久化到数据库，完整值不会再次显示。</span>
            </label>
            {tokenError && <p className="notice err">{tokenError}</p>}
            <div className="dialog-actions">
              <Button onClick={() => void saveToken()} disabled={!tokenInput.trim() || tokenSaving}>
                <KeyRound size={14} />
                {tokenSaving ? '正在保存…' : '更新 Token'}
              </Button>
            </div>
          </Panel>
          <Panel title="日志与数据" subtitle="控制审计数据的保留策略">
            {dataFields.map(spec => (
              <NumericField key={spec.key} spec={spec} value={form[spec.key]} onChange={setValue(spec.key)} />
            ))}
          </Panel>
          <Panel title="运行环境" subtitle="由环境变量控制，修改后需重启服务">
            <label className="field">
              <span className="field-label">监听地址</span>
              <input value={settings?.listen_addr ?? ''} readOnly className="readonly" />
            </label>
            <label className="field">
              <span className="field-label">数据库路径</span>
              <input value={settings?.database_path ?? ''} readOnly className="readonly" />
            </label>
          </Panel>
        </div>
      </div>
      {saved && (
        <div className="toasts">
          <Toast kind="ok" onClose={() => setSaved(false)}>
            配置已保存并立即生效
          </Toast>
        </div>
      )}
      {tokenSaved && (
        <div className="toasts">
          <Toast kind="ok" onClose={() => setTokenSaved(false)}>
            Cursor Token 已更新并立即生效
          </Toast>
        </div>
      )}
    </>
  )
}
