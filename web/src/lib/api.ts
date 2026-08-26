export class ApiError extends Error {
  constructor(
    public readonly code: string,
    public readonly status: number,
  ) {
    super(code)
    this.name = 'ApiError'
  }
}

export async function api<T = void>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(path, {
    ...init,
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', ...init.headers },
  })
  if (!response.ok) {
    let code = 'request_failed'
    try {
      const body = (await response.json()) as { error?: string }
      code = body.error || code
    } catch {
      // keep the fallback code
    }
    throw new ApiError(code, response.status)
  }
  if (response.status === 204) return undefined as T
  return (await response.json()) as T
}
