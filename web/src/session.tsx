import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { api } from './lib/api'
import type { LoginInput } from './lib/types'

type SessionValue = {
  ready: boolean
  ok: boolean
  login: (input: LoginInput) => Promise<void>
  logout: () => Promise<void>
}

const Session = createContext<SessionValue>({
  ready: false,
  ok: false,
  login: async () => undefined,
  logout: async () => undefined,
})

export function useSession() {
  return useContext(Session)
}

export function SessionProvider({ children }: { children: ReactNode }) {
  const [ready, setReady] = useState(false)
  const [ok, setOk] = useState(false)

  useEffect(() => {
    api<void>('/admin/session')
      .then(() => setOk(true))
      .catch(() => setOk(false))
      .finally(() => setReady(true))
  }, [])

  const login = async (input: LoginInput) => {
    await api<void>('/admin/session', { method: 'POST', body: JSON.stringify(input) })
    setOk(true)
  }

  const logout = async () => {
    await api<void>('/admin/session', { method: 'DELETE' }).catch(() => undefined)
    setOk(false)
  }

  return <Session.Provider value={{ ready, ok, login, logout }}>{children}</Session.Provider>
}
