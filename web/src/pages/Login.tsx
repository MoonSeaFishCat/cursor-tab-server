import { useEffect, useState, type FormEvent } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { Database, RefreshCw, ShieldCheck, Zap } from 'lucide-react'
import { api } from '../lib/api'
import type { Captcha } from '../lib/types'
import { useSession } from '../session'
import { Button } from '../components/ui/button'

export function LoginPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [answer, setAnswer] = useState('')
  const [captcha, setCaptcha] = useState<Captcha>()
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const { ready, ok, login } = useSession()
  const navigate = useNavigate()

  const loadCaptcha = async () => {
    setError('')
    setCaptcha(undefined)
    try {
      setCaptcha(await api<Captcha>('/admin/captcha'))
    } catch {
      setError('验证码暂时无法加载，请刷新后重试。')
    }
  }

  useEffect(() => {
    void loadCaptcha()
  }, [])

  if (ready && ok) return <Navigate to="/" />

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!captcha) return
    setLoading(true)
    try {
      await login({ username, password, captcha_id: captcha.captcha_id, captcha_answer: answer })
      navigate('/')
    } catch {
      setAnswer('')
      await loadCaptcha()
      setError('用户名、密码或验证码无效')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login">
      <section className="login-frame">
        <div className="login-aside">
          <span className="eyebrow on-dark">Secure Access</span>
          <h1>Cursor Tab 服务控制台</h1>
          <p>通过管理员账号、密码和一次性图形验证码，管理代理调用、密钥与运行状态。</p>
          <ul className="login-points">
            <li>
              <ShieldCheck size={15} />
              <span>
                <strong>8 小时安全会话</strong>
                <small>HttpOnly + Secure Cookie</small>
              </span>
            </li>
            <li>
              <Database size={15} />
              <span>
                <strong>SQLite 审计</strong>
                <small>全量调用元数据留痕</small>
              </span>
            </li>
            <li>
              <Zap size={15} />
              <span>
                <strong>独立限流</strong>
                <small>代理与管理面相互隔离</small>
              </span>
            </li>
          </ul>
        </div>
        <form className="login-card" onSubmit={event => void submit(event)}>
          <span className="eyebrow">管理员登录</span>
          <h2>欢迎回来</h2>
          <label className="field">
            <span className="field-label">用户名</span>
            <input aria-label="用户名" value={username} onChange={event => setUsername(event.target.value)} autoFocus required autoComplete="username" />
          </label>
          <label className="field">
            <span className="field-label">密码</span>
            <input aria-label="密码" type="password" value={password} onChange={event => setPassword(event.target.value)} required autoComplete="current-password" />
          </label>
          <div className="field">
            <span className="field-label">图形验证码</span>
            <div className="captcha-box">
              {captcha ? (
                <img src={captcha.image} alt="图形验证码" className="captcha-img" />
              ) : (
                <div className="captcha-empty">加载中…</div>
              )}
              <Button variant="secondary" size="small" type="button" onClick={() => void loadCaptcha()}>
                <RefreshCw size={14} />
                刷新
              </Button>
            </div>
          </div>
          <label className="field">
            <span className="field-label">验证码</span>
            <input aria-label="验证码" value={answer} onChange={event => setAnswer(event.target.value.toUpperCase())} required autoComplete="off" maxLength={6} />
          </label>
          {error && <p className="notice err">{error}</p>}
          <Button className="login-submit" disabled={!captcha || loading}>
            {loading ? '正在验证…' : '登录'}
          </Button>
        </form>
      </section>
    </div>
  )
}
