import { useEffect, type ElementType, type ReactNode } from 'react'
import { AlertCircle, CheckCircle2, Inbox, Loader2, X } from 'lucide-react'

export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
}: {
  eyebrow: string
  title: string
  description: string
  actions?: ReactNode
}) {
  return (
    <header className="page-head">
      <div>
        <span className="eyebrow">{eyebrow}</span>
        <h1 className="page-title">{title}</h1>
        <p className="page-desc">{description}</p>
      </div>
      {actions && <div className="page-actions">{actions}</div>}
    </header>
  )
}

export function Panel({
  title,
  subtitle,
  actions,
  children,
  className = '',
}: {
  title?: string
  subtitle?: string
  actions?: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <section className={`panel ${className}`.trim()}>
      {(title || actions) && (
        <div className="panel-head">
          <div>
            {title && <h2 className="panel-title">{title}</h2>}
            {subtitle && <p className="panel-sub">{subtitle}</p>}
          </div>
          {actions}
        </div>
      )}
      {children}
    </section>
  )
}

export function MetricCard({
  icon: Icon,
  label,
  value,
  detail,
  tone = 'default',
}: {
  icon: ElementType
  label: string
  value: string
  detail: string
  tone?: 'default' | 'ok' | 'bad'
}) {
  return (
    <section className={`metric tone-${tone}`}>
      <div className="metric-top">
        <span className="metric-label">{label}</span>
        <span className="metric-icon">
          <Icon size={16} />
        </span>
      </div>
      <strong className="metric-value">{value}</strong>
      <span className="metric-sub">{detail}</span>
    </section>
  )
}

export function Badge({ tone, children }: { tone: 'ok' | 'bad' | 'warn' | 'neutral'; children: ReactNode }) {
  return <span className={`badge ${tone}`}>{children}</span>
}

export function StatusBadge({ code }: { code: number }) {
  const tone = code >= 500 ? 'bad' : code >= 400 ? 'warn' : 'ok'
  return <Badge tone={tone}>{code}</Badge>
}

export function Dialog({
  title,
  description,
  onClose,
  children,
}: {
  title: string
  description?: string
  onClose: () => void
  children: ReactNode
}) {
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  return (
    <div className="overlay" role="presentation" onMouseDown={event => event.target === event.currentTarget && onClose()}>
      <div className="dialog" role="dialog" aria-modal="true" aria-label={title}>
        <div className="dialog-head">
          <div>
            <h2>{title}</h2>
            {description && <p className="muted">{description}</p>}
          </div>
          <button className="button ghost sm" aria-label="关闭" onClick={onClose}>
            <X size={16} />
          </button>
        </div>
        {children}
      </div>
    </div>
  )
}

export function EmptyState({ text, hint }: { text: string; hint?: string }) {
  return (
    <div className="empty">
      <Inbox size={22} />
      <strong>{text}</strong>
      {hint && <span>{hint}</span>}
    </div>
  )
}

export function Splash() {
  return (
    <div className="splash">
      <Loader2 className="spin" size={22} />
      <span>正在载入控制台…</span>
    </div>
  )
}

export function Toast({ kind, children, onClose }: { kind: 'ok' | 'err'; children: ReactNode; onClose: () => void }) {
  useEffect(() => {
    const timer = window.setTimeout(onClose, 3600)
    return () => window.clearTimeout(timer)
  }, [onClose])
  const Icon = kind === 'ok' ? CheckCircle2 : AlertCircle
  return (
    <div className={`toast ${kind}`} role="status">
      <Icon size={16} />
      <span>{children}</span>
    </div>
  )
}
