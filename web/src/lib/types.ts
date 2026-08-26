export type ApiKeyActivity = {
  api_key_id: string
  requests_24h: number
  errors_24h: number
  average_latency_ms: number
  last_status_code: number
}

export type ApiKey = {
  id: string
  name: string
  prefix: string
  created_at: string
  disabled_at?: string | null
  last_used_at?: string | null
  activity: ApiKeyActivity
}

export type AuditLog = {
  id: number
  occurred_at: string
  api_key_id?: string
  source_ip: string
  method: string
  path: string
  status_code: number
  duration_ms: number
  request_bytes: number
  response_bytes: number
  error_kind?: string
}

export type Captcha = {
  captcha_id: string
  image: string
  expires_at: string
}

export type LoginInput = {
  username: string
  password: string
  captcha_id: string
  captcha_answer: string
}

export type StatusCount = { status_code: number; count: number }

export type Dashboard = {
  requests_24h: number
  errors_24h: number
  average_latency_ms: number
  success_rate: number
  active_keys_24h: number
  status_distribution: StatusCount[]
  server: { started_at: string; listen_addr: string }
}

export type SystemSettings = {
  listen_addr: string
  database_path: string
  proxy_rate_per_minute: number
  admin_rate_per_minute: number
  captcha_rate_per_minute: number
  login_rate_per_minute: number
  log_retention_days: number
  cursor_token_set: boolean
  cursor_token_masked: string
}

export type ServiceStatus = {
  database: string
  started_at: string
  proxy_rate_per_minute: number
  admin_rate_per_minute: number
  log_retention_days: number
}

export type ListResponse<T> = { items: T[]; total: number; limit: number; offset: number }
