import { Skeleton } from '@/components/ui/Skeleton'
import { PhotoViewMode } from '@/types/photo'

export function LoadingState({ viewMode = 'grid' }: { viewMode?: PhotoViewMode }) {
  if (viewMode === 'list') {
    return (
      <div className="space-y-3">
        {Array.from({ length: 6 }).map((_, i) => (
          <div
            key={i}
            className="flex items-center justify-between p-3 rounded-xl border border-border bg-card"
          >
            <div className="flex items-center gap-4">
              <Skeleton className="w-16 h-16 rounded-lg" />
              <div className="space-y-2">
                <Skeleton className="h-4 w-40" />
                <Skeleton className="h-3 w-24" />
              </div>
            </div>
            <Skeleton className="h-4 w-20" />
          </div>
        ))}
      </div>
    )
  }

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 md:gap-6">
      {Array.from({ length: 8 }).map((_, i) => (
        <div key={i} className="flex flex-col rounded-xl overflow-hidden border border-border bg-card">
          <Skeleton className="aspect-[4/3] w-full" />
          <div className="p-3.5 space-y-2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-3 w-1/2" />
          </div>
        </div>
      ))}
    </div>
  )
}
