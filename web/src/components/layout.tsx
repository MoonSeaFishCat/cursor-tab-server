import { NavLink, useLocation } from 'react-router-dom'
import type { ReactNode } from 'react'
import { Activity, FileClock, Gauge, KeyRound, LogOut, Network, Settings2, ShieldCheck } from 'lucide-react'
import { useSession } from '../session'

const sections = [
  {
    label: '总览',
    items: [
      { to: '/', label: '仪表盘', icon: Gauge, end: true },
      { to: '/status', label: '服务状态', icon: Activity },
    ],
  },
  {
    label: '接入',
    items: [
      { to: '/api-keys', label: 'API 密钥', icon: KeyRound },
      { to: '/tokens', label: 'Token 池', icon: Network },
      { to: '/audit-logs', label: '审计日志', icon: FileClock },
    ],
  },
  {
    label: '系统',
    items: [{ to: '/settings', label: '系统配置', icon: Settings2 }],
  },
]

const titles: Record<string, string> = {
  '/': '仪表盘',
  '/status': '服务状态',
  '/api-keys': 'API 密钥',
  '/tokens': 'Token 池',
  '/audit-logs': '审计日志',
  '/settings': '系统配置',
}

export function AppShell({ children }: { children: ReactNode }) {
  const { logout } = useSession()
  const location = useLocation()
  const current = titles[location.pathname] ?? (location.pathname.startsWith('/api-keys/') ? 'API Key 详情' : '控制台')

  return (
    <div className="app">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">
            <ShieldCheck size={17} />
          </span>
          <span className="brand-text">
            <strong>Cursor Tab</strong>
            <small>Server Console</small>
          </span>
        </div>
        <nav className="nav">
          {sections.map(section => (
            <div className="nav-section" key={section.label}>
              <span className="nav-label">{section.label}</span>
              {section.items.map(({ to, label, icon: Icon, end }) => (
                <NavLink key={to} to={to} end={end} className="nav-link">
                  <Icon size={15} />
                  {label}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>
        <div className="sidebar-foot">
          <div className="session-chip">
            <span className="pulse" aria-hidden="true" />
            <div>
              <strong>安全会话已建立</strong>
              <small>8 小时后自动过期</small>
            </div>
          </div>
          <button className="button ghost sidebar-logout" onClick={() => void logout()}>
            <LogOut size={14} />
            退出登录
          </button>
        </div>
      </aside>
      <div className="workspace">
        <header className="topbar">
          <nav className="crumb" aria-label="面包屑">
            <span>Console</span>
            <span className="crumb-sep">/</span>
            <strong>{current}</strong>
          </nav>
          <div className="topbar-actions">
            <span className="env-badge">
              <span className="pulse" aria-hidden="true" />
              服务运行中
            </span>
          </div>
        </header>
        <main className="content">{children}</main>
      </div>
    </div>
  )
}
