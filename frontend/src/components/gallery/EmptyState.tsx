import { Images, SearchX } from 'lucide-react'
import { Button } from '@/components/ui/Button'

interface EmptyStateProps {
  isSearch?: boolean
  searchQuery?: string
  onReset?: () => void
  onUploadClick?: () => void
}

export function EmptyState({ isSearch, searchQuery, onReset, onUploadClick }: EmptyStateProps) {
  if (isSearch) {
    return (
      <div className="flex flex-col items-center justify-center p-12 text-center border border-dashed border-border rounded-2xl bg-card/40 my-8">
        <div className="flex items-center justify-center w-12 h-12 rounded-full bg-muted text-muted-foreground mb-4">
          <SearchX className="w-6 h-6 stroke-[1.5]" />
        </div>
        <h3 className="text-base font-semibold text-foreground mb-1">No matching photos found</h3>
        <p className="text-sm text-muted-foreground max-w-sm mb-4">
          No photos matched &ldquo;{searchQuery}&rdquo;. Try searching for different keywords, cameras, or tags.
        </p>
        {onReset && (
          <Button variant="outline" size="sm" onClick={onReset}>
            Clear search
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
      <p className="text-sm text-muted-foreground max-w-sm mb-5 leading-relaxed">
        Upload your first photograph to start curating your personal gallery collection.
      </p>
      {onUploadClick && (
        <Button onClick={onUploadClick} size="md">
          Upload Photos
        </Button>
      )}
    </div>
  )
}
