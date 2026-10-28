import { Images, SearchX, RefreshCw, UploadCloud } from 'lucide-react'
import { Button } from '@/components/ui/Button'

interface EmptyStateProps {
  isSearch?: boolean
  searchQuery?: string
  onReset?: () => void
  onUploadClick?: () => void
  onScanClick?: () => void
  isScanning?: boolean
}

export function EmptyState({
  isSearch,
  searchQuery,
  onReset,
  onUploadClick,
  onScanClick,
  isScanning,
}: EmptyStateProps) {
  if (isSearch) {
    return (
      <div className="flex flex-col items-center justify-center p-12 text-center border border-dashed border-border rounded-2xl bg-card/40 my-8">
        <div className="flex items-center justify-center w-12 h-12 rounded-full bg-muted text-muted-foreground mb-4">
          <SearchX className="w-6 h-6 stroke-[1.5]" />
        </div>
        <h3 className="text-base font-semibold text-foreground mb-1">No matching photos found</h3>
        <p className="text-sm text-muted-foreground max-w-sm mb-4">
          {searchQuery ? (
            <>No photos matched &ldquo;{searchQuery}&rdquo;. Try adjusting your keywords or clearing active filters.</>
          ) : (
            <>No photos matched the active filter criteria. Try adjusting your filter settings or date range.</>
          )}
        </p>
        {onReset && (
          <Button variant="outline" size="sm" onClick={onReset}>
            Reset search &amp; filters
          </Button>
        )}
      </div>
    )
  }

  return (
    <div className="flex flex-col items-center justify-center p-14 text-center border border-dashed border-border rounded-2xl bg-card/40 my-8">
      <div className="flex items-center justify-center w-14 h-14 rounded-full bg-primary/10 text-primary mb-4">
        <Images className="w-7 h-7 stroke-[1.5]" />
      </div>
      <h3 className="text-lg font-semibold text-foreground mb-1">Your gallery is empty</h3>
      <p className="text-sm text-muted-foreground max-w-md mb-6 leading-relaxed">
        Upload photos directly or scan existing images from the mounted media directory to populate your gallery.
      </p>

      <div className="flex flex-wrap items-center justify-center gap-3">
        {onUploadClick && (
          <Button onClick={onUploadClick} size="md" className="gap-2">
            <UploadCloud className="w-4 h-4" />
            <span>Upload Photos</span>
          </Button>
        )}

        {onScanClick && (
          <Button
            variant="outline"
            size="md"
            onClick={onScanClick}
            disabled={isScanning}
            className="gap-2"
          >
            <RefreshCw className={`w-4 h-4 ${isScanning ? 'animate-spin text-primary' : ''}`} />
            <span>{isScanning ? 'Scanning Media...' : 'Scan Existing Media'}</span>
          </Button>
        )}
      </div>
    </div>
  )
}
