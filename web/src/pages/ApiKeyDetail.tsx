import { ArrowLeft, RefreshCw } from 'lucide-react'
import { Link, useParams } from 'react-router-dom'
import { useCallback, useEffect, useState } from 'react'
import { Badge, EmptyState, PageHeader, Panel, StatusBadge } from '../components/kit'
import { Button } from '../components/ui/button'
import { api } from '../lib/api'
import { formatDateTime, formatNumber } from '../lib/format'
import type { ApiKeyDetail } from '../lib/types'

function formatBytes(bytes: number) {
  if (!bytes) return '—'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export function ApiKeyDetailPage() {
  const { id = '' } = useParams()
  const [detail, setDetail] = useState<ApiKeyDetail>()
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      setDetail(await api<ApiKeyDetail>(`/admin/api-keys/${id}?limit=25&offset=${offset}`))
    } catch {
      setError('无法加载 API Key 详情，请检查服务连接后重试。')
    } finally {
      setLoading(false)
    }
  }, [id, offset])

  useEffect(() => { void load() }, [load])

  return (
    <>
      <PageHeader
        eyebrow="Access / Detail"
        title="API Key 详情"
        description={detail ? `${detail.name} · ${detail.prefix}` : '查看 API Key 的使用统计和调用记录。'}
        actions={<><Button asChild variant="secondary"><Link to="/api-keys"><ArrowLeft size={14} />返回列表</Link></Button><Button variant="secondary" onClick={() => void load()} disabled={loading}><RefreshCw size={14} className={loading ? 'spin' : undefined} />刷新</Button></>}
      />
      {error && <p className="notice err page-notice">{error}</p>}
      {loading ? <div className="empty">正在加载 API Key 详情…</div> : detail ? <>
        <Panel title="基本信息" subtitle="密钥明文不会在详情页或接口中显示">
          <dl className="kv"><dt>名称</dt><dd>{detail.name}</dd><dt>前缀</dt><dd className="mono">{detail.prefix}</dd><dt>状态</dt><dd>{detail.disabled_at ? <Badge tone="bad">已停用</Badge> : <Badge tone="ok">启用</Badge>}</dd><dt>创建时间</dt><dd>{formatDateTime(detail.created_at)}</dd><dt>最近使用</dt><dd>{formatDateTime(detail.last_used_at)}</dd></dl>
        </Panel>
        <section className="metrics detail-metrics"><section className="metric"><span className="metric-label">24h 请求</span><strong className="metric-value">{formatNumber(detail.activity?.requests_24h ?? 0)}</strong></section><section className="metric"><span className="metric-label">24h 错误</span><strong className="metric-value">{formatNumber(detail.activity?.errors_24h ?? 0)}</strong></section><section className="metric"><span className="metric-label">平均延迟</span><strong className="metric-value">{detail.activity?.average_latency_ms ?? 0}<small className="metric-unit">ms</small></strong></section><section className="metric"><span className="metric-label">最近状态</span><strong className="metric-value">{detail.activity?.last_status_code || '—'}</strong></section></section>
        <Panel className="table-panel" title="调用记录" subtitle={`${detail.logs_total ?? 0} 条记录`}>
          {detail.logs.length === 0 ? <EmptyState text="暂无调用记录" hint="该 API Key 产生代理调用后会显示在这里" /> : <><div className="table-wrap"><table><thead><tr><th>时间</th><th>方法</th><th>路径</th><th>状态</th><th>耗时</th><th>流量</th><th>来源</th></tr></thead><tbody>{detail.logs.map(log => <tr key={log.id}><td>{formatDateTime(log.occurred_at)}</td><td className="mono">{log.method}</td><td className="mono">{log.path}</td><td><StatusBadge code={log.status_code} /></td><td className="mono">{log.duration_ms} ms</td><td className="mono">{formatBytes(log.request_bytes)} / {formatBytes(log.response_bytes)}</td><td className="mono">{log.source_ip}</td></tr>)}</tbody></table></div><div className="pagination"><span>第 {offset + 1}–{offset + detail.logs.length} 条，共 {detail.logs_total} 条</span><div><Button variant="secondary" size="small" onClick={() => setOffset(Math.max(0, offset - 25))} disabled={offset === 0 || loading}>上一页</Button><Button variant="secondary" size="small" onClick={() => setOffset(offset + 25)} disabled={offset + detail.logs.length >= detail.logs_total || loading}>下一页</Button></div></div></>}
        </Panel>
      </> : null}
    </>
  )
}
