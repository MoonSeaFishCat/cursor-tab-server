import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { RotateCcw, Save } from 'lucide-react'
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
  const [allowAnonymousProxy, setAllowAnonymousProxy] = useState(false)
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

  const applySettings = (value: SystemSettings) => {
    setSettings(value)
    setAllowAnonymousProxy(value.allow_anonymous_proxy)
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
    return (Object.keys(form) as NumericKey[]).some(key => form[key] !== String(settings[key])) || allowAnonymousProxy !== settings.allow_anonymous_proxy
  }, [form, settings, allowAnonymousProxy])

  const numericDirty = useMemo(() => {
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
    const payload: Partial<Record<NumericKey, number>> & { allow_anonymous_proxy?: boolean } = {}
    for (const key of Object.keys(form) as NumericKey[]) {
      payload[key] = Number(form[key])
    }
    payload.allow_anonymous_proxy = allowAnonymousProxy
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
            <Button onClick={() => void save()} disabled={!dirty || (numericDirty && invalid) || saving}>
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
          <Panel title="访问策略" subtitle="控制代理是否接受不带 API Key 的请求">
          <label className="switch-field"><input type="checkbox" aria-label="允许无 API Key 访问代理" checked={allowAnonymousProxy} disabled={!settings || saving} onChange={event => setAllowAnonymousProxy(event.target.checked)} /><span><strong>允许无 API Key 访问代理</strong><small>开启后按来源 IP 限流，并以匿名请求写入审计日志。默认关闭。</small></span></label>
        </Panel>
        <Panel title="Cursor 凭证" subtitle="代理转发所用的上游凭证由 Token 池统一调度">
            <p className="muted">Cursor Token 由独立凭证池统一管理。</p>
            <p className="field-hint">可在凭证池中添加多个加密 Token、查看健康状态，并随时启用或停用。</p>
            <div className="dialog-actions">
              <Button asChild>
                <Link to="/tokens">管理 Token 池</Link>
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
    </>
  )
}
