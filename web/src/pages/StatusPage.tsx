import { useCallback, useEffect, useState } from 'react'
import { Database, Gauge, RefreshCw, Timer, Zap } from 'lucide-react'
import { api } from '../lib/api'
import { formatDateTime, formatUptime } from '../lib/format'
import type { ServiceStatus } from '../lib/types'
import { Badge, MetricCard, PageHeader, Panel } from '../components/kit'
import { Button } from '../components/ui/button'

export function StatusPage() {
  const [status, setStatus] = useState<ServiceStatus>()
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [checkedAt, setCheckedAt] = useState<Date>()
  const [now, setNow] = useState(() => Date.now())

  const load = useCallback(async () => {
    setLoading(true); setError('')
    try { setStatus(await api<ServiceStatus>('/admin/status')); setCheckedAt(new Date()) } catch { setError('无法读取服务状态，请检查服务是否仍在运行。') } finally { setLoading(false) }
  }, [])
  useEffect(() => { void load(); const timer = window.setInterval(() => setNow(Date.now()), 30000); return () => window.clearInterval(timer) }, [load])

  return <><PageHeader eyebrow="Runtime" title="服务状态" description="检查数据库连接、运行时长和当前实际生效的限流配置。" actions={<Button variant="secondary" onClick={() => void load()} disabled={loading}><RefreshCw size={14} className={loading ? 'spin' : undefined} />重新检查</Button>} />
    {error && <p className="notice err page-notice">{error}</p>}<p className="refresh-meta">{checkedAt ? `最后检查：${formatDateTime(checkedAt.toISOString())}` : '正在检查服务状态…'}</p>
    <section className="metrics"><MetricCard icon={Database} label="数据库" value={error ? '异常' : status ? '正常' : '—'} detail={error || 'SQLite 连接状态'} tone={error ? 'bad' : 'ok'} /><MetricCard icon={Timer} label="运行时长" value={status ? formatUptime(status.started_at, now) : '—'} detail={status ? `启动于 ${formatDateTime(status.started_at)}` : '正在读取'} /><MetricCard icon={Zap} label="代理限流" value={status ? `${status.proxy_rate_per_minute} 次/分钟` : '—'} detail="每个密钥 + IP" /><MetricCard icon={Gauge} label="管理限流" value={status ? `${status.admin_rate_per_minute} 次/分钟` : '—'} detail="每个来源 IP" /></section>
    <section className="panel-grid"><Panel title="实例详情" subtitle="当前进程的状态与在线配置"><dl className="kv"><dt>数据库</dt><dd>{error ? <Badge tone="bad">不可用</Badge> : <Badge tone="ok">{status?.database ?? '正在检测'}</Badge>}</dd><dt>Redis 协调</dt><dd>{status?.redis === 'connected' ? <Badge tone="ok">已连接</Badge> : <Badge tone="warn">本地降级</Badge>}</dd><dt>Cursor Token</dt><dd>{status ? `${status.enabled_cursor_tokens} / ${status.cursor_tokens} 个启用` : '—'}</dd><dt>启动时间</dt><dd>{formatDateTime(status?.started_at)}</dd><dt>日志保留</dt><dd>{status ? `${status.log_retention_days} 天` : '—'}</dd><dt>状态来源</dt><dd>实时健康检查</dd></dl></Panel><Panel title="配置生效规则" subtitle="避免把需要重启的配置误认为已更新"><p className="muted">代理、管理、验证码和登录限流，以及日志保留策略均支持在线更新。Cursor Token 可在凭证池中在线增减和停用；监听地址、数据库路径与 Redis 地址由启动环境决定，修改后需要重启服务。</p></Panel></section>
  </>
}
