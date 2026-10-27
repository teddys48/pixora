import { Search, X, LayoutGrid, List, ArrowDownUp } from 'lucide-react'
import { PhotoSortOption, PhotoViewMode } from '@/types/photo'
import { cn } from '@/lib/utils'

interface GalleryControlsProps {
  searchQuery: string
  onSearchChange: (query: string) => void
  sortOption: PhotoSortOption
  onSortChange: (option: PhotoSortOption) => void
  viewMode: PhotoViewMode
  onViewModeChange: (mode: PhotoViewMode) => void
  totalCount: number
}

export function GalleryControls({
  searchQuery,
  onSearchChange,
  sortOption,
  onSortChange,
  viewMode,
  onViewModeChange,
  totalCount,
}: GalleryControlsProps) {
  return (
    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-6 pb-4 border-b border-border/60">
      {/* Search Input */}
      <div className="relative flex-1 max-w-sm">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => onSearchChange(e.target.value)}
          placeholder="Search by title, tag, or camera..."
          className="w-full h-9 pl-9 pr-8 text-sm rounded-lg border border-input bg-card/60 placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
        />
        {searchQuery && (
          <button
            onClick={() => onSearchChange('')}
            aria-label="Clear search"
            className="absolute right-2.5 top-1/2 -translate-y-1/2 p-0.5 rounded text-muted-foreground hover:text-foreground"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        )}
      </div>

      {/* Sorting, View mode, and Counter */}
      <div className="flex items-center gap-3 self-end sm:self-auto">
        {/* Count */}
        <span className="text-xs font-medium text-muted-foreground hidden lg:inline">
          {totalCount} {totalCount === 1 ? 'photo' : 'photos'}
        </span>

        {/* Sort Select */}
        <div className="relative flex items-center">
          <ArrowDownUp className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground pointer-events-none" />
          <select
            value={sortOption}
            onChange={(e) => onSortChange(e.target.value as PhotoSortOption)}
            aria-label="Sort photos"
            className="h-9 pl-8 pr-3 text-xs font-medium rounded-lg border border-input bg-card/60 text-foreground appearance-none cursor-pointer focus:outline-none focus:ring-1 focus:ring-ring"
          >
            <option value="date-desc">Newest capture</option>
            <option value="date-asc">Oldest capture</option>
            <option value="title-asc">Title (A-Z)</option>
            <option value="title-desc">Title (Z-A)</option>
            <option value="size-desc">Largest file</option>
          </select>
        </div>

        {/* View Mode Toggle */}
        <div
          role="group"
          aria-label="View mode"
          className="flex items-center rounded-lg border border-input bg-card/60 p-0.5"
        >
          <button
            onClick={() => onViewModeChange('grid')}
            aria-label="Grid view"
            aria-pressed={viewMode === 'grid'}
            className={cn(
              'p-1.5 rounded-md text-xs transition-colors',
              viewMode === 'grid'
                ? 'bg-primary text-primary-foreground shadow-xs'
                : 'text-muted-foreground hover:text-foreground'
            )}
          >
            <LayoutGrid className="h-4 w-4" />
          </button>
          <button
            onClick={() => onViewModeChange('list')}
            aria-label="List view"
            aria-pressed={viewMode === 'list'}
            className={cn(
              'p-1.5 rounded-md text-xs transition-colors',
              viewMode === 'list'
                ? 'bg-primary text-primary-foreground shadow-xs'
                : 'text-muted-foreground hover:text-foreground'
            )}
          >
            <List className="h-4 w-4" />
          </button>
        </div>
      </div>
    </div>
  )
}
