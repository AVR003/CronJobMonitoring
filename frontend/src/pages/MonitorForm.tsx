import { useEffect, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { api } from '../api/client'

type MonitorType = 'ping' | 'tcp' | 'http' | 'postgres'

interface FieldDef {
  key: string
  label: string
  type: 'text' | 'number' | 'password' | 'url'
  placeholder?: string
  required?: boolean
}

const TYPE_FIELDS: Record<MonitorType, FieldDef[]> = {
  ping: [
    { key: 'host',       label: 'Host / IP',      type: 'text',   required: true,  placeholder: '192.168.1.1' },
    { key: 'count',      label: 'Ping Count',      type: 'number', placeholder: '3' },
    { key: 'timeout_ms', label: 'Timeout (ms)',    type: 'number', placeholder: '1000' },
  ],
  tcp: [
    { key: 'host', label: 'Host / IP', type: 'text',   required: true },
    { key: 'port', label: 'Port',      type: 'number', required: true, placeholder: '22' },
  ],
  http: [
    { key: 'url',             label: 'URL',                   type: 'url',    required: true, placeholder: 'https://example.com' },
    { key: 'method',          label: 'Method',                type: 'text',   placeholder: 'GET' },
    { key: 'expected_status', label: 'Expected Status Code',  type: 'number', placeholder: '200' },
    { key: 'body_pattern',    label: 'Body Pattern (regex)',   type: 'text',   placeholder: 'optional' },
  ],
  postgres: [
    { key: 'host',       label: 'Host',                           type: 'text',     required: true },
    { key: 'port',       label: 'Port',                           type: 'number',   placeholder: '5432' },
    { key: 'database',   label: 'Database',                       type: 'text',     required: true },
    { key: 'user',       label: 'User',                           type: 'text',     required: true },
    { key: 'password',   label: 'Password (dev / plaintext)',     type: 'password' },
    { key: 'vault_path', label: 'Vault Path (overrides password)',type: 'text',     placeholder: 'monitors/my-db' },
    { key: 'query',      label: 'Probe Query',                    type: 'text',     placeholder: 'SELECT 1' },
  ],
}

const MONITOR_TYPES: MonitorType[] = ['ping', 'tcp', 'http', 'postgres']

type ConfigValue = string | number

export default function MonitorForm() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const isEdit = !!id

  const [name, setName]               = useState('')
  const [description, setDescription] = useState('')
  const [monitorType, setMonitorType] = useState<MonitorType>('ping')
  const [intervalSecs, setInterval]   = useState(60)
  const [timeoutSecs, setTimeout_]    = useState(10)
  const [config, setConfig]           = useState<Record<string, ConfigValue>>({})
  const [saving, setSaving]           = useState(false)
  const [error, setError]             = useState('')

  useEffect(() => {
    if (!id) return
    api.getMonitor(id).then(m => {
      setName(m.name)
      setDescription(m.description)
      setMonitorType(m.monitor_type as MonitorType)
      setInterval(m.interval_secs)
      setTimeout_(m.timeout_secs)
      setConfig(m.config as Record<string, ConfigValue>)
    }).catch(e => setError((e as Error).message))
  }, [id])

  function handleConfigChange(key: string, value: string, fieldType: string) {
    setConfig(prev => ({
      ...prev,
      [key]: fieldType === 'number' && value !== '' ? Number(value) : value,
    }))
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setSaving(true)
    setError('')
    try {
      const payload = { name, description, monitor_type: monitorType, interval_secs: intervalSecs, timeout_secs: timeoutSecs, config }
      if (isEdit) {
        await api.updateMonitor(id!, payload)
      } else {
        await api.createMonitor(payload)
      }
      navigate('/')
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  const fields = TYPE_FIELDS[monitorType] ?? []

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b px-6 py-4 flex items-center gap-4">
        <Link to="/" className="text-gray-400 hover:text-gray-600 text-sm">← Back</Link>
        <h1 className="text-xl font-semibold text-gray-900">
          {isEdit ? 'Edit Monitor' : 'New Monitor'}
        </h1>
      </header>

      <div className="p-6 max-w-xl mx-auto">
        <form onSubmit={handleSubmit} className="space-y-4">

          <div className="bg-white rounded-xl border p-5 space-y-4">
            <h2 className="font-medium text-gray-900">Basic Info</h2>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Name *</label>
              <input
                type="text" value={name} onChange={e => setName(e.target.value)} required
                placeholder="My server ping"
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
              <input
                type="text" value={description} onChange={e => setDescription(e.target.value)}
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Check every (sec)</label>
                <input
                  type="number" value={intervalSecs} min={10}
                  onChange={e => setInterval(Number(e.target.value))}
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Timeout (sec)</label>
                <input
                  type="number" value={timeoutSecs} min={1}
                  onChange={e => setTimeout_(Number(e.target.value))}
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
            </div>
          </div>

          <div className="bg-white rounded-xl border p-5 space-y-4">
            <h2 className="font-medium text-gray-900">Monitor Type</h2>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
              {MONITOR_TYPES.map(t => (
                <button
                  key={t} type="button"
                  onClick={() => { setMonitorType(t); setConfig({}) }}
                  className={`py-2 px-3 rounded-lg border text-sm font-medium transition-colors ${
                    monitorType === t
                      ? 'bg-blue-600 text-white border-blue-600'
                      : 'border-gray-300 text-gray-700 hover:bg-gray-50'
                  }`}
                >
                  {t.toUpperCase()}
                </button>
              ))}
            </div>

            {fields.length > 0 && (
              <div className="space-y-3 pt-2 border-t">
                {fields.map(f => (
                  <div key={f.key}>
                    <label className="block text-sm font-medium text-gray-700 mb-1">
                      {f.label}{f.required ? ' *' : ''}
                    </label>
                    <input
                      type={f.type}
                      value={(config[f.key] as string | number | undefined) ?? ''}
                      onChange={e => handleConfigChange(f.key, e.target.value, f.type)}
                      required={f.required}
                      placeholder={f.placeholder}
                      className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                    />
                  </div>
                ))}
              </div>
            )}
          </div>

          {error && (
            <div className="bg-red-50 text-red-700 border border-red-200 rounded-lg p-3 text-sm">{error}</div>
          )}

          <div className="flex gap-3">
            <button
              type="submit" disabled={saving}
              className="flex-1 bg-blue-600 text-white py-2 rounded-lg font-medium hover:bg-blue-700 disabled:opacity-50 transition-colors"
            >
              {saving ? 'Saving…' : isEdit ? 'Update Monitor' : 'Create Monitor'}
            </button>
            <Link
              to="/"
              className="px-5 py-2 border border-gray-300 rounded-lg text-gray-700 hover:bg-gray-50 text-center text-sm"
            >
              Cancel
            </Link>
          </div>
        </form>
      </div>
    </div>
  )
}
