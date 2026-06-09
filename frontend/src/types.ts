export interface Monitor {
  id: string
  name: string
  description: string
  monitor_type: string
  enabled: boolean
  interval_secs: number
  timeout_secs: number
  config: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CheckResult {
  id: number
  monitor_id: string
  checked_at: string
  status: 'up' | 'down' | 'degraded' | 'unknown'
  latency_ms?: number
  detail?: Record<string, unknown>
  error_message?: string
}

export interface MonitorStatus {
  monitor_id: string
  name: string
  monitor_type: string
  enabled: boolean
  last_result?: CheckResult
}

export interface APIToken {
  id: string
  name: string
  created_at: string
  expires_at?: string
  enabled: boolean
}
