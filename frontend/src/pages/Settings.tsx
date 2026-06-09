import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import type { APIToken } from '../types'

export default function Settings() {
  const [tokens, setTokens]       = useState<APIToken[]>([])
  const [newName, setNewName]     = useState('')
  const [newToken, setNewToken]   = useState('')
  const [creating, setCreating]   = useState(false)
  const [error, setError]         = useState('')

  useEffect(() => {
    api.getTokens().then(setTokens).catch(e => setError((e as Error).message))
  }, [])

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    setCreating(true)
    setError('')
    try {
      const result = await api.createToken(newName)
      setNewToken(result.token)
      setNewName('')
      setTokens(prev => [{ id: result.id, name: result.name, created_at: result.created_at, enabled: true }, ...prev])
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setCreating(false)
    }
  }

  async function handleRevoke(id: string, name: string) {
    if (!confirm(`Revoke token "${name}"?`)) return
    try {
      await api.revokeToken(id)
      setTokens(prev => prev.filter(t => t.id !== id))
    } catch (e) {
      setError((e as Error).message)
    }
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b px-6 py-4 flex items-center gap-4">
        <Link to="/" className="text-gray-400 hover:text-gray-600 text-sm">← Back</Link>
        <h1 className="text-xl font-semibold text-gray-900">Settings</h1>
      </header>

      <div className="p-6 max-w-2xl mx-auto space-y-6">
        {error && (
          <div className="bg-red-50 text-red-700 border border-red-200 rounded-lg p-3 text-sm">{error}</div>
        )}

        {newToken && (
          <div className="bg-green-50 border border-green-200 rounded-xl p-4">
            <p className="text-sm font-medium text-green-800 mb-2">Token created — copy it now, it won't be shown again</p>
            <code className="block bg-white border border-green-300 rounded p-3 text-sm font-mono break-all select-all">
              {newToken}
            </code>
            <button onClick={() => setNewToken('')} className="mt-2 text-xs text-green-700 hover:underline">
              Dismiss
            </button>
          </div>
        )}

        <div className="bg-white rounded-xl border p-5">
          <h2 className="font-medium text-gray-900 mb-4">API Tokens</h2>

          <form onSubmit={handleCreate} className="flex gap-2 mb-5">
            <input
              type="text" value={newName} onChange={e => setNewName(e.target.value)}
              placeholder="Token name (e.g. grafana)" required
              className="flex-1 border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <button
              type="submit" disabled={creating || !newName.trim()}
              className="bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
            >
              {creating ? 'Creating…' : 'Generate'}
            </button>
          </form>

          <div className="divide-y">
            {tokens.length === 0 ? (
              <p className="text-sm text-gray-400 py-4 text-center">No tokens yet</p>
            ) : tokens.map(t => (
              <div key={t.id} className="flex items-center justify-between py-3">
                <div>
                  <p className="text-sm font-medium text-gray-800">{t.name}</p>
                  <p className="text-xs text-gray-400">{new Date(t.created_at).toLocaleDateString()}</p>
                </div>
                <button
                  onClick={() => handleRevoke(t.id, t.name)}
                  className="text-xs text-red-500 hover:text-red-700 border border-red-200 px-2.5 py-1 rounded-lg hover:bg-red-50"
                >
                  Revoke
                </button>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
