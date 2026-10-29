import { useEffect, useRef } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Photo, PhotoSortOption } from '@/types/photo'
import { PhotoCard } from '@/components/gallery/PhotoCard'
import { GalleryControls } from '@/components/gallery/GalleryControls'
import { EmptyState } from '@/components/gallery/EmptyState'
import { LoadingState } from '@/components/gallery/LoadingState'
import { Button } from '@/components/ui/Button'
import { usePhotos } from '@/context/PhotosContext'
import { useInfiniteScroll } from '@/hooks/useInfiniteScroll'
import { InfiniteScrollFooter } from '@/components/gallery/InfiniteScrollFooter'
import { AlertCircle, CheckCircle2 } from 'lucide-react'

interface PhotosPageProps {
  onSelectPhoto: (photo: Photo, photosList: Photo[]) => void
  onUploadClick: () => void
}

export function PhotosPage({
  onSelectPhoto,
  onUploadClick,
}: PhotosPageProps) {
  const [searchParams, setSearchParams] = useSearchParams()
  const isInitializedRef = useRef(false)

  const {
    photos,
    totalPhotos,
    hasMore,
    isLoading,
    isLoadingMore,
    error,
    loadMoreError,
    sortOption,
    setSortOption,
    viewMode,
    setViewMode,
    searchQuery,
    setSearchQuery,
    startDate,
    setStartDate,
    endDate,
    setEndDate,
    folderFilter,
    setFolderFilter,
    albumFilter,
    setAlbumFilter,
    mediaTypeFilter,
    setMediaTypeFilter,
    undatedOnly,
    setUndatedOnly,
    hasActiveFilters,
    activeFiltersCount,
    clearFilters,
    resetAll,
    isScanning,
    scanResult,
    setScanResult,
    loadPhotos,
    loadMore,
    handleScan,
  } = usePhotos()

  const { sentinelRef } = useInfiniteScroll({
    onLoadMore: loadMore,
    hasMore,
    isLoading: isLoadingMore,
    disabled: Boolean(loadMoreError),
  })

  // 1. Initial mount: read URL search parameters into state
  useEffect(() => {
    if (isInitializedRef.current) return
    isInitializedRef.current = true

    const q = searchParams.get('q')
    const start = searchParams.get('startDate')
    const end = searchParams.get('endDate')
    const folder = searchParams.get('folder')
    const album = searchParams.get('albumId')
    const media = searchParams.get('mediaType')
    const undated = searchParams.get('undated')
    const sort = searchParams.get('sort') as PhotoSortOption | null

    if (q) setSearchQuery(q)
    if (start) setStartDate(start)
    if (end) setEndDate(end)
    if (folder) setFolderFilter(folder)
    if (album) setAlbumFilter(album)
    if (media) setMediaTypeFilter(media)
    if (undated === 'true') setUndatedOnly(true)
    if (
      sort &&
      [
        'date-desc',
        'date-asc',
        'title-asc',
        'title-desc',
        'size-desc',
        'size-asc',
        'modified-desc',
        'modified-asc',
      ].includes(sort)
    ) {
      setSortOption(sort)
    }
  }, [
    searchParams,
    setSearchQuery,
    setStartDate,
    setEndDate,
    setFolderFilter,
    setAlbumFilter,
    setMediaTypeFilter,
    setUndatedOnly,
    setSortOption,
  ])

  // 2. Synchronize search & discovery filter state back to URL query parameters
  useEffect(() => {
    if (!isInitializedRef.current) return

    const params = new URLSearchParams()
    if (searchQuery.trim()) params.set('q', searchQuery.trim())
    if (startDate) params.set('startDate', startDate)
    if (endDate) params.set('endDate', endDate)
    if (folderFilter) params.set('folder', folderFilter)
    if (albumFilter) params.set('albumId', albumFilter)
    if (mediaTypeFilter) params.set('mediaType', mediaTypeFilter)
    if (undatedOnly) params.set('undated', 'true')
    if (sortOption !== 'date-desc') params.set('sort', sortOption)

    if (params.toString() !== searchParams.toString()) {
      setSearchParams(params, { replace: true })
    }
  }, [
    searchQuery,
    startDate,
    endDate,
    folderFilter,
    albumFilter,
    mediaTypeFilter,
    undatedOnly,
    sortOption,
    searchParams,
    setSearchParams,
  ])

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex flex-col sm:flex-row sm:items-end justify-between gap-2">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">
            {hasActiveFilters ? 'Filtered Photos' : 'All Photos'}
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            {hasActiveFilters
              ? 'Photos matching your active search and discovery criteria.'
              : 'Browse, search, and curate your photographic collection.'}
          </p>
        </div>

        {hasActiveFilters && (
          <button
            onClick={resetAll}
            className="text-xs text-primary hover:underline font-medium self-start sm:self-auto"
          >
            &larr; Reset all filters
          </button>
        )}
      </div>

      {/* Scan Summary Banner */}
      {scanResult && (
        <div className="flex items-center justify-between p-3.5 rounded-xl border border-primary/20 bg-primary/5 text-xs text-foreground animate-in fade-in duration-200">
          <div className="flex items-center gap-2.5">
            <CheckCircle2 className="w-4 h-4 text-primary shrink-0" />
            <span>
              Scan completed: <strong>{scanResult.indexed}</strong> indexed,{' '}
              <strong>{scanResult.skipped}</strong> unchanged,{' '}
              <strong>{scanResult.updated}</strong> updated
              {scanResult.failed > 0 && `, ${scanResult.failed} failed`}.
            </span>
          </div>
          <button
            onClick={() => setScanResult(null)}
            className="text-muted-foreground hover:text-foreground text-xs font-medium ml-3"
          >
            Dismiss
          </button>
        </div>
      )}

      {/* Error Alert */}
      {error && (
        <div className="flex items-center justify-between p-3.5 rounded-xl border border-destructive/20 bg-destructive/10 text-destructive text-xs">
          <div className="flex items-center gap-2">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{error}</span>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => loadPhotos(true)}
            className="h-7 text-xs ml-3"
          >
            Retry
          </Button>
        </div>
      )}

      {/* Controls Bar */}
      <GalleryControls
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        sortOption={sortOption}
        onSortChange={setSortOption}
        viewMode={viewMode}
        onViewModeChange={setViewMode}
        totalCount={totalPhotos}
        onScanClick={handleScan}
        isScanning={isScanning}
        startDate={startDate}
        onStartDateChange={setStartDate}
        endDate={endDate}
        onEndDateChange={setEndDate}
        folderFilter={folderFilter}
        onFolderFilterChange={setFolderFilter}
        albumFilter={albumFilter}
        onAlbumFilterChange={setAlbumFilter}
        mediaTypeFilter={mediaTypeFilter}
        onMediaTypeFilterChange={setMediaTypeFilter}
        undatedOnly={undatedOnly}
        onUndatedOnlyChange={setUndatedOnly}
        hasActiveFilters={hasActiveFilters}
        activeFiltersCount={activeFiltersCount}
        onClearFilters={clearFilters}
        onResetAll={resetAll}
      />

      {/* Gallery Content Area */}
      {isLoading ? (
        <LoadingState viewMode={viewMode} />
      ) : photos.length === 0 ? (
        <EmptyState
          isSearch={hasActiveFilters || Boolean(searchQuery.trim())}
          searchQuery={searchQuery}
          onReset={resetAll}
          onUploadClick={onUploadClick}
          onScanClick={handleScan}
          isScanning={isScanning}
        />
      ) : viewMode === 'grid' ? (
        <>
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 md:gap-6">
            {photos.map((photo) => (
              <PhotoCard
                key={photo.id}
                photo={photo}
                onSelect={(p) => onSelectPhoto(p, photos)}
                viewMode="grid"
              />
            ))}
          </div>

          <InfiniteScrollFooter
            sentinelRef={sentinelRef}
            hasMore={hasMore}
            isLoadingMore={isLoadingMore}
            error={loadMoreError}
            onRetry={loadMore}
            loadedCount={photos.length}
            totalCount={totalPhotos}
          />
        </>
      ) : (
        <>
          <div className="space-y-2.5">
            {photos.map((photo) => (
              <PhotoCard
                key={photo.id}
                photo={photo}
                onSelect={(p) => onSelectPhoto(p, photos)}
                viewMode="list"
              />
            ))}
          </div>

          <InfiniteScrollFooter
            sentinelRef={sentinelRef}
            hasMore={hasMore}
            isLoadingMore={isLoadingMore}
            error={loadMoreError}
            onRetry={loadMore}
            loadedCount={photos.length}
            totalCount={totalPhotos}
          />
        </>
      )}
    </div>
  )
}
