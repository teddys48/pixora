import React from 'react'
import { Photo, PhotoViewMode } from '@/types/photo'
import { FallbackImage } from '@/components/ui/FallbackImage'
import { formatDate, formatBytes } from '@/lib/utils'
import { Camera, Maximize2, X } from 'lucide-react'

interface PhotoCardProps {
  photo: Photo
  onSelect: (photo: Photo) => void
  viewMode?: PhotoViewMode
  onRemove?: (photo: Photo) => void
  removeLabel?: string
}

export function PhotoCard({
  photo,
  onSelect,
  viewMode = 'grid',
  onRemove,
  removeLabel = 'Remove',
}: PhotoCardProps) {
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      onSelect(photo)
    }
  }

  const isCaptureDate = Boolean(photo.takenAt && !photo.isUndated)
  const hasFallbackDate = Boolean(!isCaptureDate && photo.fileModifiedAt)
  const dateTitle = isCaptureDate
    ? 'Verified EXIF capture date'
    : hasFallbackDate
    ? 'Filesystem modification time (fallback, no EXIF capture date)'
    : 'No reliable date available'
  const displayDate = isCaptureDate
    ? formatDate(photo.takenAt)
    : hasFallbackDate
    ? `${formatDate(photo.fileModifiedAt)}`
    : 'Undated'

  if (viewMode === 'list') {
    return (
      <div
        id={`photo-${photo.id}`}
        role="button"
        tabIndex={0}
        onClick={() => onSelect(photo)}
        onKeyDown={handleKeyDown}
        className="group flex items-center justify-between p-3 rounded-xl border border-border bg-card hover:bg-muted/40 transition-all cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <div className="flex items-center gap-4 min-w-0">
          <div className="w-16 h-16 sm:w-20 sm:h-20 rounded-lg overflow-hidden shrink-0 border border-border/50 bg-muted/40 flex items-center justify-center">
            <FallbackImage
              src={photo.thumbnailUrl}
              alt={photo.title}
              aspectRatio="square"
              className="w-full h-full flex items-center justify-center"
              imgClassName="w-full h-full object-contain group-hover:scale-105 transition-transform duration-300"
            />
          </div>
          <div className="min-w-0">
            <h3 className="font-semibold text-sm text-foreground truncate group-hover:text-primary transition-colors">
              {photo.title}
            </h3>
            <p className="text-xs text-muted-foreground truncate mt-0.5">{photo.filename}</p>
            <div className="flex items-center gap-3 mt-1.5 text-xs text-muted-foreground">
              <span
                title={dateTitle}
                className={hasFallbackDate ? 'italic text-muted-foreground/90' : undefined}
              >
                {displayDate}
              </span>
              {photo.metadata.camera && (
                <span className="hidden sm:flex items-center gap-1 truncate">
                  <Camera className="w-3 h-3" />
                  {photo.metadata.camera}
                </span>
              )}
            </div>
          </div>
        </div>

        <div className="flex items-center gap-3 text-xs text-muted-foreground shrink-0 pl-2">
          <span className="hidden md:inline font-mono">
            {photo.metadata.width} × {photo.metadata.height}
          </span>
          <span className="font-mono">{formatBytes(photo.size)}</span>
          {onRemove && (
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation()
                onRemove(photo)
              }}
              aria-label={removeLabel}
              title={removeLabel}
              className="p-1.5 rounded-md hover:bg-destructive/10 text-muted-foreground hover:text-destructive transition-colors"
            >
              <X className="w-4 h-4" />
            </button>
          )}
          <button
            type="button"
            aria-label="View photo"
            className="p-1.5 rounded-md hover:bg-muted text-muted-foreground group-hover:text-foreground"
          >
            <Maximize2 className="w-4 h-4" />
          </button>
        </div>
      </div>
    )
  }

  return (
    <div
      id={`photo-${photo.id}`}
      role="button"
      tabIndex={0}
      onClick={() => onSelect(photo)}
      onKeyDown={handleKeyDown}
      className="group relative flex flex-col rounded-xl overflow-hidden border border-border/70 bg-card hover:border-border transition-all duration-300 hover:shadow-lg cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      {/* Thumbnail Container: uniform 4:3 card with object-contain to preserve original aspect ratio */}
      <div className="relative aspect-[4/3] w-full overflow-hidden bg-muted/40 flex items-center justify-center">
        <FallbackImage
          src={photo.thumbnailUrl}
          alt={photo.title}
          className="w-full h-full flex items-center justify-center"
          imgClassName="w-full h-full object-contain group-hover:scale-105 transition-transform duration-500 ease-out"
        />

        {/* Hover overlay with quick preview indicator */}
        <div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity duration-300 flex items-center justify-center p-3 pointer-events-none">
          <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-black/70 text-white text-xs font-medium backdrop-blur-sm shadow-sm">
            <Maximize2 className="w-3.5 h-3.5" />
            <span>View</span>
          </div>
        </div>

        {/* Remove Action Button (when in album view) */}
        {onRemove && (
          <div className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity z-10">
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation()
                onRemove(photo)
              }}
              title={removeLabel}
              aria-label={removeLabel}
              className="p-1.5 rounded-full bg-black/70 hover:bg-destructive text-white backdrop-blur-sm transition-colors shadow-sm"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          </div>
        )}

        {/* Size Badge */}
        <div className="absolute bottom-2 right-2 opacity-90 group-hover:opacity-100 transition-opacity">
          <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-black/60 text-white backdrop-blur-sm">
            {formatBytes(photo.size)}
          </span>
        </div>
      </div>

      {/* Card Info */}
      <div className="p-3.5 flex flex-col justify-between flex-1">
        <div>
          <h3 className="font-medium text-sm text-foreground truncate group-hover:text-primary transition-colors">
            {photo.title}
          </h3>
          <p className="text-xs text-muted-foreground/80 truncate mt-0.5 font-mono">
            {photo.filename}
          </p>
        </div>

        <div className="flex items-center justify-between text-[11px] text-muted-foreground mt-3 pt-2.5 border-t border-border/50">
          <span
            title={dateTitle}
            className={hasFallbackDate ? 'italic text-muted-foreground/90' : undefined}
          >
            {displayDate}
          </span>
          {photo.metadata.camera ? (
            <span className="truncate max-w-[120px] opacity-80" title={photo.metadata.camera}>
              {photo.metadata.camera.replace('Sony ', '').replace('Fujifilm ', '').replace('Canon EOS ', '')}
            </span>
          ) : (
            <span className="font-mono opacity-80">
              {photo.metadata.width}×{photo.metadata.height}
            </span>
          )}
        </div>
      </div>
    </div>
  )
}
