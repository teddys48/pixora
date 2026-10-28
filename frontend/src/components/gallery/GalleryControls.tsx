import { useState, useEffect } from 'react'
import {
  Search,
  X,
  LayoutGrid,
  List,
  ArrowDownUp,
  RefreshCw,
  SlidersHorizontal,
  Folder as FolderIcon,
  FolderOpen,
  Calendar,
  FileType,
  HelpCircle,
  RotateCcw,
} from 'lucide-react'
import { PhotoSortOption, PhotoViewMode } from '@/types/photo'
import { Folder, PhysicalFolder } from '@/types/folder'
import { fetchAlbums, fetchPhysicalFolders } from '@/lib/api'
import { cn, formatDate } from '@/lib/utils'
import { DateRangeModal } from '@/components/gallery/DateRangeModal'

interface GalleryControlsProps {
  searchQuery: string
  onSearchChange: (query: string) => void
  sortOption: PhotoSortOption
  onSortChange: (option: PhotoSortOption) => void
  viewMode: PhotoViewMode
  onViewModeChange: (mode: PhotoViewMode) => void
  totalCount: number
  onScanClick?: () => void
  isScanning?: boolean
  // Extended Discovery Filters
  startDate?: string
  onStartDateChange?: (date: string) => void
  endDate?: string
  onEndDateChange?: (date: string) => void
  folderFilter?: string
  onFolderFilterChange?: (folder: string) => void
  albumFilter?: string
  onAlbumFilterChange?: (albumId: string) => void
  mediaTypeFilter?: string
  onMediaTypeFilterChange?: (type: string) => void
  undatedOnly?: boolean
  onUndatedOnlyChange?: (undated: boolean) => void
  hasActiveFilters?: boolean
  activeFiltersCount?: number
  onClearFilters?: () => void
  onResetAll?: () => void
}

export function GalleryControls({
  searchQuery,
  onSearchChange,
  sortOption,
  onSortChange,
  viewMode,
  onViewModeChange,
  totalCount,
  onScanClick,
  isScanning,
  startDate = '',
  onStartDateChange,
  endDate = '',
  onEndDateChange,
  folderFilter = '',
  onFolderFilterChange,
  albumFilter = '',
  onAlbumFilterChange,
  mediaTypeFilter = '',
  onMediaTypeFilterChange,
  undatedOnly = false,
  onUndatedOnlyChange,
  hasActiveFilters = false,
  activeFiltersCount = 0,
  onClearFilters,
  onResetAll,
}: GalleryControlsProps) {
  const [isFilterPanelOpen, setIsFilterPanelOpen] = useState(false)
  const [isDateModalOpen, setIsDateModalOpen] = useState(false)
  const [availableAlbums, setAvailableAlbums] = useState<Folder[]>([])
  const [availableFolders, setAvailableFolders] = useState<PhysicalFolder[]>([])

  // Load available albums and physical folders for dropdown options
  useEffect(() => {
    let isMounted = true
    const loadDropdownData = async () => {
      try {
        const [albums, foldersData] = await Promise.all([
          fetchAlbums().catch(() => []),
          fetchPhysicalFolders().catch(() => ({ folders: [] })),
        ])
        if (isMounted) {
          setAvailableAlbums(albums)
          setAvailableFolders(foldersData.folders || [])
        }
      } catch {
        // Fallback gracefully
      }
    }
    loadDropdownData()
    return () => {
      isMounted = false
    }
  }, [])

  // Auto-expand filter panel if specific filters are active on load
  useEffect(() => {
    if (startDate || endDate || folderFilter || albumFilter || mediaTypeFilter || undatedOnly) {
      setIsFilterPanelOpen(true)
    }
  }, [startDate, endDate, folderFilter, albumFilter, mediaTypeFilter, undatedOnly])

  const isDateRangeInvalid = Boolean(startDate && endDate && startDate > endDate)

  const activeAlbumName = availableAlbums.find((a) => a.id === albumFilter)?.name || albumFilter

  return (
    <div className="space-y-3 mb-6 pb-4 border-b border-border/60">
      {/* Primary Toolbar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        {/* Search Input */}
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder="Search by filename or folder..."
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

        {/* Action Controls */}
        <div className="flex items-center gap-2.5 self-end sm:self-auto flex-wrap">
          {/* Filter Toggle Button */}
          <button
            type="button"
            onClick={() => setIsFilterPanelOpen((prev) => !prev)}
            aria-label="Toggle filter options"
            aria-expanded={isFilterPanelOpen}
            className={cn(
              'flex items-center gap-1.5 h-9 px-3 rounded-lg border text-xs font-medium transition-colors',
              isFilterPanelOpen || hasActiveFilters
                ? 'border-primary/50 bg-primary/10 text-primary hover:bg-primary/15'
                : 'border-input bg-card/60 text-muted-foreground hover:text-foreground hover:bg-muted/50'
            )}
          >
            <SlidersHorizontal className="h-3.5 w-3.5" />
            <span>Filters</span>
            {activeFiltersCount > 0 && (
              <span className="flex items-center justify-center w-4 h-4 text-[10px] font-bold rounded-full bg-primary text-primary-foreground ml-0.5">
                {activeFiltersCount}
              </span>
            )}
          </button>

          {/* Sort Selector */}
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
              <option value="title-asc">Filename (A-Z)</option>
              <option value="title-desc">Filename (Z-A)</option>
              <option value="size-desc">Largest file</option>
              <option value="size-asc">Smallest file</option>
              <option value="modified-desc">Newest modified</option>
              <option value="modified-asc">Oldest modified</option>
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

          {/* Scan Media Button */}
          {onScanClick && (
            <button
              type="button"
              onClick={onScanClick}
              disabled={isScanning}
              aria-label="Scan media directory"
              title="Scan host media directory for photos"
              className="flex items-center gap-1.5 h-9 px-3 rounded-lg border border-input bg-card/60 text-xs font-medium text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors disabled:opacity-50"
            >
              <RefreshCw className={cn('h-3.5 w-3.5', isScanning && 'animate-spin text-primary')} />
              <span className="hidden sm:inline">{isScanning ? 'Scanning...' : 'Scan Media'}</span>
            </button>
          )}
        </div>
      </div>

      {/* Expandable Filter Panel */}
      {isFilterPanelOpen && (
        <div className="p-4 rounded-xl border border-border/80 bg-card/40 backdrop-blur-xs space-y-4 animate-in fade-in slide-in-from-top-2 duration-200">
          <div className="flex items-center justify-between pb-2 border-b border-border/40">
            <div className="flex items-center gap-2">
              <SlidersHorizontal className="w-4 h-4 text-primary" />
              <span className="text-xs font-semibold text-foreground tracking-tight">
                Filter Discovery Criteria
              </span>
            </div>
            {hasActiveFilters && (
              <button
                type="button"
                onClick={onClearFilters || onResetAll}
                className="text-xs text-muted-foreground hover:text-destructive flex items-center gap-1 font-medium transition-colors"
              >
                <RotateCcw className="w-3 h-3" />
                <span>Reset Filters</span>
              </button>
            )}
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3.5 text-xs">
            {/* Capture Date Range */}
            <div className="space-y-1.5">
              <div className="flex items-center justify-between">
                <label className="font-medium text-foreground flex items-center gap-1.5">
                  <Calendar className="w-3.5 h-3.5 text-muted-foreground" />
                  <span>Capture Date Range</span>
                </label>
                {(startDate || endDate) && (
                  <button
                    type="button"
                    onClick={() => {
                      onStartDateChange?.('')
                      onEndDateChange?.('')
                    }}
                    className="text-[10px] text-muted-foreground hover:text-destructive transition-colors"
                  >
                    Clear
                  </button>
                )}
              </div>

              {/* Clickable Date Range Picker Button (Opens Modal) */}
              <button
                type="button"
                onClick={() => setIsDateModalOpen(true)}
                className={cn(
                  'flex items-center justify-between w-full h-8 px-2.5 rounded-md border text-xs transition-colors text-left',
                  startDate || endDate
                    ? 'border-primary/50 bg-primary/10 text-primary font-medium'
                    : 'border-input bg-card text-muted-foreground hover:text-foreground hover:bg-muted/50'
                )}
              >
                <div className="flex items-center gap-1.5 truncate">
                  <Calendar className="w-3.5 h-3.5 shrink-0 opacity-80" />
                  <span className="truncate">
                    {startDate && endDate
                      ? `${formatDate(startDate)} – ${formatDate(endDate)}`
                      : startDate
                      ? `From ${formatDate(startDate)}`
                      : endDate
                      ? `Up to ${formatDate(endDate)}`
                      : 'Choose dates (calendar modal)...'}
                  </span>
                </div>
                <span className="text-[10px] underline ml-1 text-primary shrink-0">
                  {startDate || endDate ? 'Change' : 'Pick'}
                </span>
              </button>

              <div className="grid grid-cols-2 gap-2 pt-0.5">
                <div>
                  <input
                    type="date"
                    value={startDate}
                    onClick={() => setIsDateModalOpen(true)}
                    onChange={(e) => onStartDateChange?.(e.target.value)}
                    aria-label="Start capture date"
                    className="w-full h-7 px-1.5 rounded border border-input bg-card text-[11px] text-foreground focus:outline-none focus:ring-1 focus:ring-ring cursor-pointer"
                    title="Click to open date calendar modal"
                  />
                  <span className="text-[10px] text-muted-foreground/80 mt-0.5 block">From</span>
                </div>
                <div>
                  <input
                    type="date"
                    value={endDate}
                    onClick={() => setIsDateModalOpen(true)}
                    onChange={(e) => onEndDateChange?.(e.target.value)}
                    aria-label="End capture date"
                    className="w-full h-7 px-1.5 rounded border border-input bg-card text-[11px] text-foreground focus:outline-none focus:ring-1 focus:ring-ring cursor-pointer"
                    title="Click to open date calendar modal"
                  />
                  <span className="text-[10px] text-muted-foreground/80 mt-0.5 block">To</span>
                </div>
              </div>

              {isDateRangeInvalid && (
                <p className="text-[11px] text-destructive font-medium">
                  Start date cannot be after end date.
                </p>
              )}
            </div>

            {/* Physical Folder Filter */}
            <div className="space-y-1.5">
              <label className="font-medium text-foreground flex items-center gap-1.5">
                <FolderIcon className="w-3.5 h-3.5 text-muted-foreground" />
                <span>Physical Folder</span>
              </label>
              <select
                value={folderFilter}
                onChange={(e) => onFolderFilterChange?.(e.target.value)}
                aria-label="Filter by physical folder"
                className="w-full h-8 px-2.5 rounded-md border border-input bg-card text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
              >
                <option value="">All Physical Folders</option>
                {availableFolders.map((f) => (
                  <option key={f.path} value={f.path}>
                    {f.name} ({f.photoCount} photos)
                  </option>
                ))}
              </select>
              <span className="text-[10px] text-muted-foreground/80 block">
                Includes subfolders under selected path
              </span>
            </div>

            {/* Virtual Album Filter */}
            <div className="space-y-1.5">
              <label className="font-medium text-foreground flex items-center gap-1.5">
                <FolderOpen className="w-3.5 h-3.5 text-muted-foreground" />
                <span>Virtual Album</span>
              </label>
              <select
                value={albumFilter}
                onChange={(e) => onAlbumFilterChange?.(e.target.value)}
                aria-label="Filter by virtual album"
                className="w-full h-8 px-2.5 rounded-md border border-input bg-card text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
              >
                <option value="">All Albums</option>
                {availableAlbums.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name} ({a.photoCount} photos)
                  </option>
                ))}
              </select>
              <span className="text-[10px] text-muted-foreground/80 block">
                Virtual collections in database
              </span>
            </div>

            {/* Media Type & Undated Options */}
            <div className="space-y-1.5">
              <label className="font-medium text-foreground flex items-center gap-1.5">
                <FileType className="w-3.5 h-3.5 text-muted-foreground" />
                <span>Media Format</span>
              </label>
              <select
                value={mediaTypeFilter}
                onChange={(e) => onMediaTypeFilterChange?.(e.target.value)}
                aria-label="Filter by media type"
                className="w-full h-8 px-2.5 rounded-md border border-input bg-card text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
              >
                <option value="">All Supported Formats</option>
                <option value="image/jpeg">JPEG (.jpg, .jpeg)</option>
                <option value="image/png">PNG (.png)</option>
                <option value="image/webp">WebP (.webp)</option>
                <option value="image/gif">GIF (.gif)</option>
                <option value="image/tiff">TIFF (.tif, .tiff)</option>
              </select>

              {/* Undated Photos Only Toggle */}
              <div className="pt-1">
                <label className="flex items-center gap-2 cursor-pointer text-muted-foreground hover:text-foreground">
                  <input
                    type="checkbox"
                    checked={undatedOnly}
                    onChange={(e) => onUndatedOnlyChange?.(e.target.checked)}
                    className="rounded border-input text-primary focus:ring-primary w-3.5 h-3.5"
                  />
                  <span className="text-[11px] font-medium flex items-center gap-1">
                    <HelpCircle className="w-3 h-3 text-muted-foreground" />
                    Undated photos only
                  </span>
                </label>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Active Filter Chips Bar */}
      <div className="flex items-center justify-between flex-wrap gap-2 pt-1 text-xs">
        <div className="flex items-center flex-wrap gap-1.5">
          {searchQuery && (
            <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full bg-primary/10 text-primary border border-primary/20 text-xs">
              <Search className="w-3 h-3" />
              <span>Query: &ldquo;{searchQuery}&rdquo;</span>
              <button
                type="button"
                onClick={() => onSearchChange('')}
                className="hover:text-foreground ml-1"
                aria-label="Remove search filter"
              >
                &times;
              </button>
            </span>
          )}

          {startDate && endDate && (
            <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full bg-primary/10 text-primary border border-primary/20 text-xs">
              <Calendar className="w-3 h-3" />
              <button
                type="button"
                onClick={() => setIsDateModalOpen(true)}
                className="hover:underline font-medium"
              >
                {formatDate(startDate)} to {formatDate(endDate)}
              </button>
              <button
                type="button"
                onClick={() => {
                  onStartDateChange?.('')
                  onEndDateChange?.('')
                }}
                className="hover:text-foreground ml-1"
                aria-label="Remove date range filter"
              >
                &times;
              </button>
            </span>
          )}

          {startDate && !endDate && (
            <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full bg-primary/10 text-primary border border-primary/20 text-xs">
              <Calendar className="w-3 h-3" />
              <button
                type="button"
                onClick={() => setIsDateModalOpen(true)}
                className="hover:underline font-medium"
              >
                From {formatDate(startDate)}
              </button>
              <button
                type="button"
                onClick={() => onStartDateChange?.('')}
                className="hover:text-foreground ml-1"
                aria-label="Remove start date filter"
              >
                &times;
              </button>
            </span>
          )}

          {!startDate && endDate && (
            <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full bg-primary/10 text-primary border border-primary/20 text-xs">
              <Calendar className="w-3 h-3" />
              <button
                type="button"
                onClick={() => setIsDateModalOpen(true)}
                className="hover:underline font-medium"
              >
                Until {formatDate(endDate)}
              </button>
              <button
                type="button"
                onClick={() => onEndDateChange?.('')}
                className="hover:text-foreground ml-1"
                aria-label="Remove end date filter"
              >
                &times;
              </button>
            </span>
          )}

          {folderFilter && (
            <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full bg-primary/10 text-primary border border-primary/20 text-xs">
              <FolderIcon className="w-3 h-3" />
              <span>Folder: {folderFilter}</span>
              <button
                type="button"
                onClick={() => onFolderFilterChange?.('')}
                className="hover:text-foreground ml-1"
                aria-label="Remove folder filter"
              >
                &times;
              </button>
            </span>
          )}

          {albumFilter && (
            <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full bg-primary/10 text-primary border border-primary/20 text-xs">
              <FolderOpen className="w-3 h-3" />
              <span>Album: {activeAlbumName}</span>
              <button
                type="button"
                onClick={() => onAlbumFilterChange?.('')}
                className="hover:text-foreground ml-1"
                aria-label="Remove album filter"
              >
                &times;
              </button>
            </span>
          )}

          {mediaTypeFilter && (
            <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full bg-primary/10 text-primary border border-primary/20 text-xs">
              <FileType className="w-3 h-3" />
              <span>Format: {mediaTypeFilter.replace('image/', '').toUpperCase()}</span>
              <button
                type="button"
                onClick={() => onMediaTypeFilterChange?.('')}
                className="hover:text-foreground ml-1"
                aria-label="Remove format filter"
              >
                &times;
              </button>
            </span>
          )}

          {undatedOnly && (
            <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full bg-primary/10 text-primary border border-primary/20 text-xs">
              <HelpCircle className="w-3 h-3" />
              <span>Undated Only</span>
              <button
                type="button"
                onClick={() => onUndatedOnlyChange?.(false)}
                className="hover:text-foreground ml-1"
                aria-label="Remove undated filter"
              >
                &times;
              </button>
            </span>
          )}

          {hasActiveFilters && (
            <button
              type="button"
              onClick={onResetAll || onClearFilters}
              className="text-xs text-muted-foreground hover:text-destructive underline ml-1 font-medium transition-colors"
            >
              Clear all
            </button>
          )}
        </div>

        {/* Result Count Indicator */}
        <div className="text-xs font-medium text-muted-foreground ml-auto">
          {hasActiveFilters ? (
            <span>
              {totalCount === 0 ? 'No matching photos' : `${totalCount} matching ${totalCount === 1 ? 'photo' : 'photos'}`}
            </span>
          ) : (
            <span>
              {totalCount} {totalCount === 1 ? 'photo' : 'photos'}
            </span>
          )}
        </div>
      </div>

      {/* Date Range Modal / Popup */}
      <DateRangeModal
        isOpen={isDateModalOpen}
        onClose={() => setIsDateModalOpen(false)}
        startDate={startDate}
        endDate={endDate}
        onApply={(start, end) => {
          onStartDateChange?.(start)
          onEndDateChange?.(end)
        }}
        onClear={() => {
          onStartDateChange?.('')
          onEndDateChange?.('')
        }}
      />
    </div>
  )
}
