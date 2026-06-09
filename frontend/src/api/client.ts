import type { Monitor, MonitorStatus, CheckResult, APIToken } from '../types'

const BASE = '/api'

export function setToken(token: string) {
  sessionStorage.setItem('api_token', token)
}

export function clearToken() {
  sessionStorage.removeItem('api_token')
}

export function isAuthenticated(): boolean {
  return !!sessionStorage.getItem('api_token')
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = sessionStorage.getItem('api_token')
  const resp = await fetch(`${BASE}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...init?.headers,
    },
  })
  if (resp.status === 401) {
    clearToken()
    window.location.href = '/login'
    throw new Error('unauthorized')
  }
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: resp.statusText }))
    throw new Error((err as { error?: string }).error ?? resp.statusText)
  }
  if (resp.status === 204) return undefined as T
  return resp.json() as Promise<T>
}

export const api = {
  health: () => request<{ status: string }>('/health'),

  getStatus: () => request<MonitorStatus[]>('/status'),

  getMonitors: () => request<Monitor[]>('/monitors'),
  getMonitor: (id: string) => request<Monitor>(`/monitors/${id}`),
  createMonitor: (data: Partial<Monitor>) =>
    request<Monitor>('/monitors', { method: 'POST', body: JSON.stringify(data) }),
  updateMonitor: (id: string, data: Partial<Monitor>) =>
    request<Monitor>(`/monitors/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteMonitor: (id: string) =>
    request<void>(`/monitors/${id}`, { method: 'DELETE' }),
  toggleMonitor: (id: string) =>
    request<{ enabled: boolean }>(`/monitors/${id}/toggle`, { method: 'PATCH' }),
  checkNow: (id: string) =>
    request<CheckResult>(`/monitors/${id}/check-now`, { method: 'POST' }),
  getResults: (id: string) =>
    request<CheckResult[]>(`/monitors/${id}/results`),
  getMonitorStatus: (id: string) =>
    request<CheckResult>(`/monitors/${id}/status`),

  getTokens: () => request<APIToken[]>('/tokens'),
  createToken: (name: string) =>
    request<{ id: string; name: string; token: string; created_at: string }>(
      '/tokens',
      { method: 'POST', body: JSON.stringify({ name }) },
    ),
  revokeToken: (id: string) =>
    request<void>(`/tokens/${id}`, { method: 'DELETE' }),
}
