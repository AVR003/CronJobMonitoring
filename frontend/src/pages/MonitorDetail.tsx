import { useEffect, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { api } from '../api/client'
import StatusBadge from '../components/StatusBadge'
import type { Monitor, CheckResult } from '../types'

export default function MonitorDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [monitor, setMonitor] = useState<Monitor | null>(null)
  const [results, setResults] = useState<CheckResult[]>([])
  const [checking, setChecking] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!id) return
    api.getMonitor(id).then(setMonitor).catch(e => setError((e as Error).message))
    api.getResults(id).then(setResults).catch(() => {})
  }, [id])

  async function handleCheckNow() {
    if (!id) return
    setChecking(true)
    try {
      const result = await api.checkNow(id)
      setResults(prev => [result, ...prev])
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setChecking(false)
    }
  }

  async function handleToggle() {
    if (!id || !monitor) return
    try {
      const { enabled } = await api.toggleMonitor(id)
      setMonitor({ ...monitor, enabled })
    } catch (e) {
      setError((e as Error).message)
    }
  }

  async function handleDelete() {
    if (!id || !confirm(`Delete "${monitor?.name}"? This cannot be undone.`)) return
    try {
      await api.deleteMonitor(id)
      navigate('/')
    } catch (e) {
      setError((e as Error).message)
    }
  }

  if (!monitor) {
    return (
      <div className="p-8 text-gray-500">
        {error ? <span className="text-red-600">{error}</span> : 'Loading…'}
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b px-6 py-4 flex items-center gap-4">
        <Link to="/" className="text-gray-400 hover:text-gray-600 text-sm">← Back</Link>
        <h1 className="text-xl font-semibold text-gray-900 flex-1">{monitor.name}</h1>
        <div className="flex gap-2">
          <button
            onClick={handleCheckNow}
            disabled={checking}
            className="bg-blue-600 text-white px-3 py-1.5 rounded-lg text-sm font-medium hover:bg-blue-700 disabled:opacity-50 transition-colors"
          >
            {checking ? 'Checking…' : 'Check Now'}
          </button>
          <Link
            to={`/monitors/${id}/edit`}
            className="border border-gray-300 text-gray-700 px-3 py-1.5 rounded-lg text-sm hover:bg-gray-50"
          >
            Edit
          </Link>
          <button
            onClick={handleToggle}
            className="border border-gray-300 text-gray-700 px-3 py-1.5 rounded-lg text-sm hover:bg-gray-50"
          >
            {monitor.enabled ? 'Disable' : 'Enable'}
          </button>
          <button
            onClick={handleDelete}
            className="border border-red-200 text-red-600 px-3 py-1.5 rounded-lg text-sm hover:bg-red-50"
          >
            Delete
          </button>
        </div>
      </header>

      <div className="p-6 max-w-4xl mx-auto">
        <div className="bg-white rounded-xl border p-5 mb-6 grid grid-cols-2 sm:grid-cols-4 gap-4 text-sm">
          <div><p className="text-gray-400 text-xs uppercase mb-1">Type</p><p className="font-medium">{monitor.monitor_type}</p></div>
          <div><p className="text-gray-400 text-xs uppercase mb-1">Interval</p><p className="font-medium">{monitor.interval_secs}s</p></div>
          <div><p className="text-gray-400 text-xs uppercase mb-1">Timeout</p><p className="font-medium">{monitor.timeout_secs}s</p></div>
          <div><p className="text-gray-400 text-xs uppercase mb-1">Status</p>
            <span className={`font-medium ${monitor.enabled ? 'text-green-600' : 'text-orange-500'}`}>
              {monitor.enabled ? 'Enabled' : 'Disabled'}
            </span>
          </div>
        </div>

        {error && (
          <div className="bg-red-50 text-red-700 border border-red-200 rounded-lg p-3 mb-4 text-sm">{error}</div>
        )}

        <h2 className="text-base font-semibold text-gray-900 mb-3">Recent Results (last 100)</h2>
        <div className="bg-white rounded-xl border overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 border-b">
              <tr>
                <th className="text-left px-4 py-2.5 font-medium text-gray-500 text-xs uppercase">Time</th>
                <th className="text-left px-4 py-2.5 font-medium text-gray-500 text-xs uppercase">Status</th>
                <th className="text-left px-4 py-2.5 font-medium text-gray-500 text-xs uppercase">Latency</th>
                <th className="text-left px-4 py-2.5 font-medium text-gray-500 text-xs uppercase">Error</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {results.length === 0 ? (
                <tr>
                  <td colSpan={4} className="px-4 py-8 text-center text-gray-400">No results yet</td>
                </tr>
              ) : results.map(r => (
                <tr key={r.id} className="hover:bg-gray-50">
                  <td className="px-4 py-2.5 text-gray-500 font-mono text-xs whitespace-nowrap">
                    {new Date(r.checked_at).toLocaleString()}
                  </td>
                  <td className="px-4 py-2.5"><StatusBadge status={r.status} /></td>
                  <td className="px-4 py-2.5 text-gray-600 font-mono text-xs">
                    {r.latency_ms != null ? `${r.latency_ms.toFixed(1)} ms` : '—'}
                  </td>
                  <td className="px-4 py-2.5 text-red-500 text-xs max-w-xs truncate">
                    {r.error_message || ''}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
