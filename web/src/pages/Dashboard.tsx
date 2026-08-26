import { useCallback, useEffect, useState } from 'react'
import { NavLink } from 'react-router-dom'
import { Activity, BarChart3, Clock3, KeyRound, RefreshCw, ShieldCheck } from 'lucide-react'
import { api } from '../lib/api'
import { formatDateTime, formatNumber } from '../lib/format'
import type { AuditLog, Dashboard, ListResponse } from '../lib/types'
import { Button } from '../components/ui/button'
import { EmptyState, MetricCard, PageHeader, Panel } from '../components/kit'
import { LogTable } from './AuditLogs'

export function DashboardPage() {
  const [dashboard, setDashboard] = useState<Dashboard>()
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState('')
  const [updatedAt, setUpdatedAt] = useState<Date>()

  const load = useCallback(async () => {
    setRefreshing(true); setError('')
    try {
      const [dash, recent] = await Promise.all([api<Dashboard>('/admin/dashboard'), api<ListResponse<AuditLog>>('/admin/audit-logs?limit=6')])
      setDashboard(dash); setLogs(recent.items ?? []); setUpdatedAt(new Date())
    } catch { setError('仪表盘数据加载失败，将保留上一次成功获取的数据。') } finally { setRefreshing(false) }
  }, [])

  useEffect(() => { void load(); const timer = window.setInterval(() => void load(), 30000); return () => window.clearInterval(timer) }, [load])
  const total = Math.max(dashboard?.requests_24h ?? 0, 1)

  return <><PageHeader eyebrow="Overview" title="运营仪表盘" description="查看代理调用规模、成功率、延迟与状态分布，每 30 秒自动刷新。" actions={<Button variant="secondary" onClick={() => void load()} disabled={refreshing}><RefreshCw size={14} className={refreshing ? 'spin' : undefined} />刷新</Button>} />
    {error && <p className="notice err page-notice">{error}</p>}
    <p className="refresh-meta">{updatedAt ? `最后更新：${formatDateTime(updatedAt.toISOString())}` : '正在读取最新运行数据…'}</p>
    <section className="metrics"><MetricCard icon={BarChart3} label="24 小时请求" value={formatNumber(dashboard?.requests_24h)} detail="代理 API 调用总量" /><MetricCard icon={ShieldCheck} label="成功率" value={dashboard ? `${dashboard.success_rate.toFixed(1)}%` : '—'} detail="非错误状态占比" tone="ok" /><MetricCard icon={Clock3} label="平均延迟" value={dashboard ? `${dashboard.average_latency_ms} ms` : '—'} detail="24 小时请求耗时" /><MetricCard icon={KeyRound} label="活跃密钥" value={formatNumber(dashboard?.active_keys_24h)} detail="最近 24 小时有调用" /></section>
    <section className="panel-grid"><Panel title="状态分布" subtitle="24 小时内各状态码的请求占比">{dashboard && dashboard.status_distribution.length > 0 ? <div className="bars">{dashboard.status_distribution.map(item => <div className="bar-row" key={item.status_code}><span className="bar-label mono">{item.status_code}</span><div className="bar-track"><i className={item.status_code >= 400 ? 'bad' : 'ok'} style={{ width: `${Math.max(4, Math.min(100, (item.count * 100) / total))}%` }} /></div><strong className="bar-value mono">{item.count.toLocaleString('zh-CN')}</strong></div>)}</div> : <EmptyState text="暂无请求数据" hint="产生代理调用后这里会显示状态分布" />}</Panel><Panel title="服务信息" subtitle="当前实例的运行参数"><dl className="kv"><dt>监听地址</dt><dd className="mono">{dashboard?.server.listen_addr || '—'}</dd><dt>启动时间</dt><dd>{formatDateTime(dashboard?.server.started_at)}</dd><dt>统计窗口</dt><dd>最近 24 小时</dd><dt>数据刷新</dt><dd>每 30 秒自动刷新</dd></dl></Panel></section>
    <Panel title="最近审计记录" subtitle="最新代理调用元数据" actions={<NavLink to="/audit-logs" className="text-link">查看全部</NavLink>} className="table-panel"><LogTable logs={logs} /></Panel>
  </>
}
