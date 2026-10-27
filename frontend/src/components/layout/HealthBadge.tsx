import { useEffect, useState } from 'react'
import { fetchHealth } from '@/lib/api'
import { HealthResponse } from '@/types/health'
import { Activity, AlertCircle } from 'lucide-react'

export function HealthBadge() {
  const [health, setHealth] = useState<HealthResponse | null>(null)
  const [status, setStatus] = useState<'checking' | 'connected' | 'disconnected'>('checking')

  useEffect(() => {
    let isMounted = true
    const controller = new AbortController()

    async function check() {
      try {
        const data = await fetchHealth(controller.signal)
        if (isMounted) {
          setHealth(data)
          setStatus('connected')
        }
      } catch (err) {
        if (isMounted) {
          setStatus('disconnected')
        }
      }
    }

    check()
    // Periodic check every 30 seconds
    const interval = setInterval(check, 30000)

    return () => {
      isMounted = false
      controller.abort()
      clearInterval(interval)
    }
  }, [])

  return (
    <div className="flex items-center gap-2 px-3 py-2 rounded-lg bg-muted/40 border border-border/50 text-xs">
      <div className="flex items-center gap-1.5 flex-1 min-w-0">
        {status === 'connected' ? (
          <>
            <span className="relative flex h-2 w-2">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
              <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
            </span>
            <span className="font-medium text-foreground truncate">API Connected</span>
          </>
        ) : status === 'checking' ? (
          <>
            <Activity className="w-3 h-3 text-muted-foreground animate-spin" />
            <span className="text-muted-foreground truncate">Checking API...</span>
          </>
        ) : (
          <>
            <AlertCircle className="w-3 h-3 text-amber-500" />
            <span className="text-muted-foreground truncate">API Offline (Mock UI)</span>
          </>
        )}
      </div>

      {status === 'connected' && health && (
        <span className="text-[10px] text-muted-foreground bg-muted px-1.5 py-0.5 rounded font-mono">
          v{health.version}
        </span>
      )}
    </div>
  )
}
