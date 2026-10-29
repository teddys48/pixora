import { RefreshCw, AlertCircle, CheckCircle2 } from 'lucide-react'
import { Button } from '@/components/ui/Button'

export interface InfiniteScrollFooterProps {
  sentinelRef: (node: HTMLDivElement | null) => void
  hasMore: boolean
  isLoadingMore: boolean
  error?: string | null
  onRetry?: () => void
  loadedCount?: number
  totalCount?: number
  endMessage?: string
}

export function InfiniteScrollFooter({
  sentinelRef,
  hasMore,
  isLoadingMore,
  error,
  onRetry,
  loadedCount,
  totalCount,
  endMessage,
}: InfiniteScrollFooterProps) {
  return (
    <div className="w-full flex flex-col items-center justify-center pt-6 pb-8">
      {/* Sentinel element observed by IntersectionObserver */}
      <div
        ref={sentinelRef}
        className="h-1 w-full pointer-events-none opacity-0"
        aria-hidden="true"
      />

      {/* Loading Indicator */}
      {isLoadingMore && (
        <div
          role="status"
          aria-live="polite"
          className="flex items-center gap-2.5 px-4 py-2 text-sm text-muted-foreground bg-muted/40 rounded-full border border-border/50 animate-pulse"
        >
          <RefreshCw className="w-4 h-4 animate-spin text-primary" />
          <span>Loading more photos...</span>
        </div>
      )}

      {/* Error state with Retry action */}
      {!isLoadingMore && error && (
        <div
          role="alert"
          className="flex flex-col sm:flex-row items-center gap-3 px-5 py-3 rounded-xl bg-destructive/10 border border-destructive/20 text-destructive text-sm"
        >
          <div className="flex items-center gap-2">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{error}</span>
          </div>
          {onRetry && (
            <Button
              variant="outline"
              size="sm"
              onClick={onRetry}
              className="border-destructive/30 text-destructive hover:bg-destructive/10 hover:text-destructive h-8 px-3"
            >
              Retry
            </Button>
          )}
        </div>
      )}

      {/* Terminal End-of-Results State */}
      {!hasMore && !isLoadingMore && !error && loadedCount !== undefined && loadedCount > 0 && (
        <div className="flex items-center gap-2 text-xs text-muted-foreground/75 py-2">
          <CheckCircle2 className="w-3.5 h-3.5 text-muted-foreground/50" />
          <span>
            {endMessage ||
              (totalCount !== undefined
                ? `Showing all ${totalCount} photo${totalCount === 1 ? '' : 's'}`
                : `All ${loadedCount} photo${loadedCount === 1 ? '' : 's'} loaded`)}
          </span>
        </div>
      )}
    </div>
  )
}
