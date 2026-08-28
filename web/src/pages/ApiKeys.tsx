import { useCallback, useEffect, useMemo, useState } from 'react'
import { Check, Copy, Eye, KeyRound, Link as LinkIcon, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import { formatDateTime } from '../lib/format'
import type { ApiKey, ListResponse } from '../lib/types'
import { Button } from '../components/ui/button'
import { Badge, Dialog, EmptyState, PageHeader, Panel, Toast } from '../components/kit'

function tabServiceAddress(secret: string) {
  return `${window.location.origin}/key=${secret}`
}

type BatchAction = 'enable' | 'disable' | 'delete'

export function ApiKeysPage() {
  const [keys, setKeys] = useState<ApiKey[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [creating, setCreating] = useState(false)
  const [name, setName] = useState('')
  const [secret, setSecret] = useState('')
  const [disabling, setDisabling] = useState<ApiKey>()
  const [copied, setCopied] = useState('')
  const [error, setError] = useState('')
  const [toast, setToast] = useState('')
  const [selected, setSelected] = useState<string[]>([])
  const [batchAction, setBatchAction] = useState<BatchAction>()
  const [batchSaving, setBatchSaving] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setLoadError('')
    try {
      const result = await api<ListResponse<ApiKey>>('/admin/api-keys')
      setKeys(result.items ?? [])
      setSelected(current => current.filter(id => result.items?.some(key => key.id === id)))
    } catch {
      setLoadError('无法加载 API 密钥，请检查服务连接后重试。')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const create = async () => {
    setError('')
    try {
      const value = await api<{ secret: string }>('/admin/api-keys', { method: 'POST', body: JSON.stringify({ name: name.trim() }) })
      setSecret(value.secret)
      setName('')
      setCreating(false)
      void load()
    } catch {
      setError('创建失败，请稍后重试。')
    }
  }

  const disable = async (key: ApiKey) => {
    try {
      await api(`/admin/api-keys/${key.id}/disable`, { method: 'POST' })
      setDisabling(undefined)
      setToast(`已停用“${key.name}”`)
      void load()
    } catch {
      setDisabling(undefined)
      setToast('停用失败，请稍后重试。')
    }
  }

  const toggleSelected = (id: string) => setSelected(current => current.includes(id) ? current.filter(value => value !== id) : [...current, id])
  const allSelected = keys.length > 0 && selected.length === keys.length
  const toggleAll = () => setSelected(allSelected ? [] : keys.map(key => key.id))
  const batchTitle = batchAction === 'delete' ? '批量删除 API 密钥' : batchAction === 'enable' ? '批量启用 API 密钥' : '批量停用 API 密钥'

  const runBatch = async () => {
    if (!batchAction || selected.length === 0) return
    setBatchSaving(true)
    try {
      await api(`/admin/api-keys/batch`, { method: 'POST', body: JSON.stringify({ ids: selected, action: batchAction }) })
      setToast(`已完成 ${selected.length} 个 API 密钥的${batchAction === 'delete' ? '删除' : batchAction === 'enable' ? '启用' : '停用'}`)
      setSelected([])
      setBatchAction(undefined)
      void load()
    } catch {
      setToast('批量操作失败，请刷新后重试。')
    } finally {
      setBatchSaving(false)
    }
  }

  const copy = async (value: string, kind: 'secret' | 'address') => {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(kind)
      window.setTimeout(() => setCopied(''), 1600)
    } catch {
      setToast('浏览器未授予剪贴板权限，请手动复制。')
    }
  }

  const selectedKeys = useMemo(() => keys.filter(key => selected.includes(key.id)), [keys, selected])

  return (
    <>
      <PageHeader eyebrow="Access" title="API 密钥" description="创建、查看和管理调用 Cursor 代理接口的密钥。数据库仅保存 SHA-256 哈希。" actions={<><Button variant="secondary" onClick={() => void load()} disabled={loading}><RefreshCw size={14} className={loading ? 'spin' : undefined} />刷新</Button><Button onClick={() => { setError(''); setCreating(true) }}><Plus size={14} />创建密钥</Button></>} />
      {loadError && <p className="notice err page-notice">{loadError}</p>}
      {selected.length > 0 && <div className="batch-toolbar"><strong>已选择 {selected.length} 个</strong><Button variant="secondary" size="small" onClick={() => setBatchAction('enable')}>批量启用</Button><Button variant="secondary" size="small" onClick={() => setBatchAction('disable')}>批量停用</Button><Button variant="destructive" size="small" onClick={() => setBatchAction('delete')}><Trash2 size={14} />批量删除</Button><Button variant="ghost" size="small" onClick={() => setSelected([])}>取消选择</Button></div>}
      <Panel className="table-panel" title="密钥列表" subtitle="活动数据统计最近 24 小时的代理请求">
        {loading ? <div className="empty">正在加载密钥…</div> : keys.length === 0 ? <EmptyState text="还没有 API 密钥" hint="创建第一个密钥即可开始调用代理接口" /> : (
          <div className="table-wrap"><table><thead><tr><th className="select-cell"><input type="checkbox" aria-label="全选 API 密钥" checked={allSelected} onChange={toggleAll} /></th><th>名称</th><th>状态</th><th>24h 请求</th><th>错误</th><th>平均延迟</th><th>最近状态</th><th>最近使用</th><th /></tr></thead><tbody>
            {keys.map(key => <tr key={key.id}><td className="select-cell"><input type="checkbox" aria-label={`选择 ${key.name}`} checked={selected.includes(key.id)} onChange={() => toggleSelected(key.id)} /></td><td><Link className="text-link" to={`/api-keys/${key.id}`}>{key.name}</Link><small className="table-sub mono">{key.prefix}</small></td><td>{key.disabled_at ? <Badge tone="bad">已停用</Badge> : <Badge tone="ok">启用</Badge>}</td><td className="mono">{key.activity?.requests_24h ?? 0}</td><td className="mono">{key.activity?.errors_24h ?? 0}</td><td className="mono">{key.activity?.average_latency_ms ?? 0} ms</td><td>{key.activity?.last_status_code ? <Badge tone={key.activity.last_status_code >= 400 ? 'warn' : 'ok'}>{key.activity.last_status_code}</Badge> : '—'}</td><td>{key.last_used_at ? formatDateTime(key.last_used_at) : '从未使用'}</td><td className="row-actions"><Button asChild variant="ghost" size="small"><Link to={`/api-keys/${key.id}`}><Eye size={14} />详情</Link></Button>{key.disabled_at ? <Button variant="ghost" size="small" onClick={() => { setSelected([key.id]); setBatchAction('enable') }}>启用</Button> : <Button variant="ghost" size="small" onClick={() => setDisabling(key)}>停用</Button>}</td></tr>)}
          </tbody></table></div>
        )}
      </Panel>
      {creating && <Dialog title="创建 API 密钥" description="名称可以留空，系统会自动生成；密钥创建后只显示一次，请立即保存。" onClose={() => setCreating(false)}><label className="field"><span className="field-label">名称（可选）</span><input aria-label="名称" value={name} onChange={event => setName(event.target.value)} placeholder="留空则自动生成名称" autoFocus /></label>{error && <p className="notice err">{error}</p>}<div className="dialog-actions"><Button variant="secondary" onClick={() => setCreating(false)}>取消</Button><Button onClick={() => void create()}><KeyRound size={14} />确认创建</Button></div></Dialog>}
      {secret && <Dialog title="保存新密钥" description="请立即保存密钥和 TAB 服务地址。两者均不会再次显示。" onClose={() => setSecret('')}><p className="notice warn">完整密钥只显示一次。请保存到安全的密码管理工具中。</p><label className="field"><span className="field-label">API 密钥</span><div className="secret-box"><code>{secret}</code><Button variant="secondary" size="small" onClick={() => void copy(secret, 'secret')}>{copied === 'secret' ? <Check size={14} /> : <Copy size={14} />}{copied === 'secret' ? '已复制' : '复制'}</Button></div></label><label className="field"><span className="field-label">Cursor TAB 服务地址</span><div className="secret-box"><code>{tabServiceAddress(secret)}</code><Button variant="secondary" size="small" onClick={() => void copy(tabServiceAddress(secret), 'address')}>{copied === 'address' ? <Check size={14} /> : <LinkIcon size={14} />}{copied === 'address' ? '已复制' : '复制地址'}</Button></div><span className="field-hint">在 Cursor 设置中粘贴此地址；服务会使用路径中的密钥完成鉴权。</span></label><div className="dialog-actions"><Button onClick={() => setSecret('')}>我已保存</Button></div></Dialog>}
      {disabling && <Dialog title="停用密钥" onClose={() => setDisabling(undefined)}><p className="muted">停用后，使用 <strong>{disabling.name}</strong>（{disabling.prefix}）的所有调用将立即被拒绝。此操作不可撤销。</p><div className="dialog-actions"><Button variant="secondary" onClick={() => setDisabling(undefined)}>取消</Button><Button variant="destructive" onClick={() => void disable(disabling)}>确认停用</Button></div></Dialog>}
      {batchAction && <Dialog title={batchTitle} description={batchAction === 'delete' ? '删除后密钥不可恢复，相关审计记录会保留但不再关联到该密钥。' : undefined} onClose={() => setBatchAction(undefined)}><p className="muted">确认对 <strong>{selectedKeys.length} 个</strong> API 密钥执行{batchAction === 'delete' ? '删除' : batchAction === 'enable' ? '启用' : '停用'}操作吗？</p><div className="dialog-actions"><Button variant="secondary" onClick={() => setBatchAction(undefined)}>取消</Button><Button variant={batchAction === 'delete' || batchAction === 'disable' ? 'destructive' : 'default'} onClick={() => void runBatch()} disabled={batchSaving}>{batchAction === 'delete' ? <Trash2 size={14} /> : null}{batchSaving ? '正在处理…' : `确认${batchAction === 'delete' ? '批量删除' : batchAction === 'enable' ? '批量启用' : '批量停用'}`}</Button></div></Dialog>}
      {toast && <div className="toasts"><Toast kind={toast.includes('失败') ? 'err' : 'ok'} onClose={() => setToast('')}>{toast}</Toast></div>}
    </>
  )
}
