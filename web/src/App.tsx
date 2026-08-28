import type { ReactNode } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { AppShell } from './components/layout'
import { Splash } from './components/kit'
import { useSession, SessionProvider } from './session'
import { LoginPage } from './pages/Login'
import { DashboardPage } from './pages/Dashboard'
import { ApiKeysPage } from './pages/ApiKeys'
import { ApiKeyDetailPage } from './pages/ApiKeyDetail'
import { TokensPage } from './pages/TokensPage'
import { AuditLogsPage } from './pages/AuditLogs'
import { StatusPage } from './pages/StatusPage'
import { SettingsPage } from './pages/SettingsPage'
import './index.css'

function Protected({ children }: { children: ReactNode }) {
  const { ready, ok } = useSession()
  if (!ready) return <Splash />
  return ok ? <AppShell>{children}</AppShell> : <Navigate to="/login" />
}

export default function App() {
  return (
    <BrowserRouter>
      <SessionProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/" element={<Protected><DashboardPage /></Protected>} />
          <Route path="/api-keys" element={<Protected><ApiKeysPage /></Protected>} />
          <Route path="/api-keys/:id" element={<Protected><ApiKeyDetailPage /></Protected>} />
          <Route path="/tokens" element={<Protected><TokensPage /></Protected>} />
          <Route path="/audit-logs" element={<Protected><AuditLogsPage /></Protected>} />
          <Route path="/status" element={<Protected><StatusPage /></Protected>} />
          <Route path="/settings" element={<Protected><SettingsPage /></Protected>} />
          <Route path="*" element={<Navigate to="/" />} />
        </Routes>
      </SessionProvider>
    </BrowserRouter>
  )
}
