type Status = 'up' | 'down' | 'degraded' | 'unknown'

const styles: Record<Status, string> = {
  up:       'bg-green-100 text-green-800',
  down:     'bg-red-100 text-red-800',
  degraded: 'bg-yellow-100 text-yellow-800',
  unknown:  'bg-gray-100 text-gray-500',
}

export default function StatusBadge({ status }: { status: string }) {
  const s = (status || 'unknown') as Status
  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold ${styles[s] ?? styles.unknown}`}>
      {s.toUpperCase()}
    </span>
  )
}
