import { useCallback, useEffect, useState } from 'react'
import { CheckCircle2, CircleOff, KeyRound, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { Badge, Dialog, EmptyState, PageHeader, Panel, Toast } from '../components/kit'
import { Button } from '../components/ui/button'
import { api } from '../lib/api'
import { formatDateTime } from '../lib/format'
import type { CursorToken, CursorTokenList } from '../lib/types'

const strategyLabels: Record<string, string> = {
  sticky_least_inflight: '粘性 + 最少进行中请求',
}

export function TokensPage() {
  const [tokens, setTokens] = useState<CursorToken[]>([])
  const [redisConnected, setRedisConnected] = useState(false)
  const [strategy, setStrategy] = useState('')
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [creating, setCreating] = useState(false)
  const [name, setName] = useState('')
  const [token, setToken] = useState('')
  const [formError, setFormError] = useState('')
  const [saving, setSaving] = useState(false)
  const [toast, setToast] = useState('')
  const [pending, setPending] = useState<{ token: CursorToken; enabled: boolean }>()
  const [deleting, setDeleting] = useState<CursorToken>()

  const load = useCallback(async () => {
    setLoading(true)
    setLoadError('')
    try {
      const result = await api<CursorTokenList>('/admin/cursor-tokens')
      setTokens(result.items ?? [])
      setRedisConnected(result.redis_connected)
      setStrategy(result.strategy)
    } catch {
      setLoadError('无法加载 Cursor Token 池，请检查服务连接。')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const closeCreateDialog = () => {
    setCreating(false)
    setFormError('')
  }

  const create = async () => {
    setSaving(true)
    setFormError('')
    try {
      await api('/admin/cursor-tokens', {
        method: 'POST',
        body: JSON.stringify({ name: name.trim(), token: token.trim() }),
      })
      setCreating(false)
      setName('')
      setToken('')
      setToast('Cursor Token 已加入池并立即生效')
      void load()
    } catch {
      setFormError('Token 保存失败，请检查名称和 Token 长度。')
    } finally {
      setSaving(false)
    }
  }

  const toggle = async () => {
    if (!pending) return
    setSaving(true)
    try {
      await api(`/admin/cursor-tokens/${pending.token.id}/${pending.enabled ? 'enable' : 'disable'}`, { method: 'POST' })
      setToast(pending.enabled ? `已启用“${pending.token.name}”` : `已停用“${pending.token.name}”`)
      setPending(undefined)
      void load()
    } catch {
      setToast('操作失败，至少需要保留一个启用中的 Token。')
    } finally {
      setSaving(false)
    }
  }

  const remove = async () => {
    if (!deleting) return
    setSaving(true)
    try {
      await api(`/admin/cursor-tokens/${deleting.id}`, { method: 'DELETE' })
      setToast(`已删除“${deleting.name}”`)
      setDeleting(undefined)
      void load()
    } catch {
      setToast('删除失败；至少需要保留一个启用中的 Token。')
    } finally {
      setSaving(false)
    }
  }

  const tokenLength = token.trim().length
  const enabledCount = tokens.filter(item => item.enabled).length

  return (
    <>
      <PageHeader
        eyebrow="Upstream"
        title="Cursor Token 池"
        description="管理多个上游凭证；同一 API Key 优先保持 Token 粘性，异常时自动切换一次。"
        actions={
          <>
            <Button variant="secondary" onClick={() => void load()} disabled={loading}>
              <RefreshCw size={14} className={loading ? 'spin' : undefined} />刷新
            </Button>
            <Button onClick={() => setCreating(true)}><Plus size={14} />添加 Token</Button>
          </>
        }
      />
      {loadError && <p className="notice err page-notice">{loadError}</p>}
      <div className="pool-summary">
        <Badge tone={redisConnected ? 'ok' : 'warn'}>{redisConnected ? 'Redis 已连接' : '本地降级模式'}</Badge>
        <span>{enabledCount} 个启用 / {tokens.length} 个总计</span>
        <span>调度策略：{strategyLabels[strategy] || strategy || '粘性 + 最少进行中请求'}</span>
      </div>
      <Panel className="table-panel" title="凭证列表" subtitle="Token 明文只在首次添加时提交，数据库保存 AES-GCM 密文。">
        {loading ? <div className="empty">正在加载 Token 池…</div> : tokens.length === 0 ? (
          <EmptyState text="还没有 Cursor Token" hint="添加至少一个 Token 后代理才能转发请求" />
        ) : (
          <div className="table-wrap">
            <table>
              <thead><tr><th>名称</th><th>凭证</th><th>状态</th><th>进行中</th><th>最近使用</th><th>最近错误</th><th /></tr></thead>
              <tbody>
                {tokens.map(item => (
                  <tr key={item.id}>
                    <td><strong>{item.name}</strong><small className="table-sub mono">{item.id.slice(0, 8)}</small></td>
                    <td className="mono">{item.masked}</td>
                    <td>{item.enabled ? item.healthy ? <Badge tone="ok">健康</Badge> : <Badge tone="warn">冷却中</Badge> : <Badge tone="bad">已停用</Badge>}</td>
                    <td className="mono">{item.in_flight}</td>
                    <td>{item.last_used_at ? formatDateTime(item.last_used_at) : '从未使用'}</td>
                    <td>{item.last_error || '—'}</td>
                    <td className="row-actions"><Button variant="ghost" size="small" onClick={() => setPending({ token: item, enabled: !item.enabled })}>{item.enabled ? '停用' : '启用'}</Button><Button variant="ghost" size="small" onClick={() => setDeleting(item)}><Trash2 size={14} />删除</Button></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>
      {creating && (
        <Dialog title="添加 Cursor Token" description="Token 将加密后保存，明文不会再次显示。" onClose={closeCreateDialog}>
          <label className="field"><span className="field-label">名称</span><input aria-label="Token 名称" value={name} onChange={event => setName(event.target.value)} placeholder="例如：主账号" autoFocus /></label>
          <label className="field"><span className="field-label">Cursor Token</span><input aria-label="Cursor Token" type="password" value={token} onChange={event => setToken(event.target.value)} placeholder="粘贴 Cursor access token" autoComplete="off" /></label>
          {formError && <p className="notice err">{formError}</p>}
          <div className="dialog-actions"><Button variant="secondary" onClick={closeCreateDialog}>取消</Button><Button onClick={() => void create()} disabled={!name.trim() || tokenLength < 10 || tokenLength > 4096 || saving}><KeyRound size={14} />{saving ? '正在保存…' : '保存 Token'}</Button></div>
        </Dialog>
      )}
      {deleting && (
        <Dialog title="删除 Cursor Token" description="删除后 Token 不可恢复，数据库中的加密凭证也会被移除。" onClose={() => setDeleting(undefined)}>
          <p className="muted">确认删除 <strong>{deleting.name}</strong>（{deleting.masked}）吗？{deleting.enabled && ' 删除最后一个启用中的 Token 会被服务拒绝。'}</p>
          <div className="dialog-actions"><Button variant="secondary" onClick={() => setDeleting(undefined)}>取消</Button><Button variant="destructive" onClick={() => void remove()} disabled={saving}><Trash2 size={14} />{saving ? '正在删除…' : '确认删除'}</Button></div>
        </Dialog>
      )}
      {pending && (
        <Dialog title={pending.enabled ? '启用 Token' : '停用 Token'} onClose={() => setPending(undefined)}>
          <p className="muted">确认{pending.enabled ? '启用' : '停用'} <strong>{pending.token.name}</strong>（{pending.token.masked}）吗？</p>
          <div className="dialog-actions"><Button variant="secondary" onClick={() => setPending(undefined)}>取消</Button><Button variant={pending.enabled ? 'default' : 'destructive'} onClick={() => void toggle()} disabled={saving}>{pending.enabled ? <CheckCircle2 size={14} /> : <CircleOff size={14} />}{pending.enabled ? '确认启用' : '确认停用'}</Button></div>
        </Dialog>
      )}
      {toast && <div className="toasts"><Toast kind={toast.includes('失败') ? 'err' : 'ok'} onClose={() => setToast('')}>{toast}</Toast></div>}
    </>
  )
}
