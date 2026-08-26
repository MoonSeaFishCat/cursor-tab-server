import { useCallback, useEffect, useState } from 'react'
import { RefreshCw, Search } from 'lucide-react'
import { api } from '../lib/api'
import { formatDateTime } from '../lib/format'
import type { ApiKey, AuditLog, ListResponse } from '../lib/types'
import { Button } from '../components/ui/button'
import { EmptyState, PageHeader, Panel, StatusBadge } from '../components/kit'

const pageSize = 25

function formatBytes(bytes: number) {
  if (!bytes) return '—'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export function LogTable({ logs }: { logs: AuditLog[] }) {
  if (logs.length === 0) return <EmptyState text="暂无审计记录" hint="产生代理调用后会自动记录在这里" />
  return <div className="table-wrap"><table><thead><tr><th>时间</th><th>方法</th><th>路径</th><th>状态</th><th>耗时</th><th>流量</th><th>错误</th><th>来源</th></tr></thead><tbody>{logs.map(log => <tr key={log.id}><td>{formatDateTime(log.occurred_at)}</td><td className="mono">{log.method}</td><td className="mono">{log.path}</td><td><StatusBadge code={log.status_code} /></td><td className="mono">{log.duration_ms} ms</td><td className="mono">{formatBytes(log.request_bytes)} / {formatBytes(log.response_bytes)}</td><td>{log.error_kind || '—'}</td><td className="mono">{log.source_ip}</td></tr>)}</tbody></table></div>
}

export function AuditLogsPage() {
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [keys, setKeys] = useState<ApiKey[]>([])
  const [status, setStatus] = useState('')
  const [path, setPath] = useState('')
  const [keyID, setKeyID] = useState('')
  const [offset, setOffset] = useState(0)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    setLoading(true); setError('')
    const params = new URLSearchParams({ limit: String(pageSize), offset: String(offset) })
    if (status) params.set('status', status)
    if (path.trim()) params.set('path', path.trim())
    if (keyID) params.set('key_id', keyID)
    try {
      const result = await api<ListResponse<AuditLog>>(`/admin/audit-logs?${params}`)
      setLogs(result.items ?? []); setTotal(result.total ?? 0)
    } catch { setError('无法加载审计日志，请检查服务连接后重试。') } finally { setLoading(false) }
  }, [status, path, keyID, offset])

  useEffect(() => { void load() }, [load])
  useEffect(() => { api<ListResponse<ApiKey>>('/admin/api-keys').then(result => setKeys(result.items ?? [])).catch(() => {}) }, [])
  const setFilter = (setter: (value: string) => void) => (value: string) => { setter(value); setOffset(0) }
  const first = total === 0 ? 0 : offset + 1
  const last = Math.min(offset + pageSize, total)

  return <><PageHeader eyebrow="Audit" title="审计日志" description="检索代理调用元数据；不会保存请求体、响应体或密钥。" actions={<Button variant="secondary" onClick={() => void load()} disabled={loading}><RefreshCw size={14} className={loading ? 'spin' : undefined} />刷新</Button>} />
    {error && <p className="notice err page-notice">{error}</p>}
    <Panel className="table-panel" title="调用记录" subtitle={total ? `显示第 ${first}–${last} 条，共 ${total} 条记录` : '暂无匹配记录'}><div className="filters"><label className="filter"><span>密钥</span><select aria-label="API 密钥" value={keyID} onChange={event => setFilter(setKeyID)(event.target.value)}><option value="">全部</option>{keys.map(key => <option key={key.id} value={key.id}>{key.name}（{key.prefix}）</option>)}</select></label><label className="filter"><span>状态码</span><select aria-label="状态码" value={status} onChange={event => setFilter(setStatus)(event.target.value)}><option value="">全部</option><option value="200">200</option><option value="401">401</option><option value="429">429</option><option value="502">502</option></select></label><form className="filter search" onSubmit={event => { event.preventDefault(); setOffset(0); void load() }}><span>路径</span><input aria-label="路径" value={path} onChange={event => setPath(event.target.value)} placeholder="例如 /aiserver.v1..." /><Button variant="secondary" size="small" type="submit"><Search size={14} />查询</Button></form></div>{loading ? <div className="empty">正在加载审计记录…</div> : <LogTable logs={logs} />}<div className="pagination"><span>{total} 条记录</span><div><Button variant="secondary" size="small" onClick={() => setOffset(Math.max(0, offset - pageSize))} disabled={offset === 0 || loading}>上一页</Button><Button variant="secondary" size="small" onClick={() => setOffset(offset + pageSize)} disabled={offset + pageSize >= total || loading}>下一页</Button></div></div></Panel></>
}
