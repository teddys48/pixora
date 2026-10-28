import { useMemo } from 'react'
import { Photo } from '@/types/photo'
import { PhotoCard } from '@/components/gallery/PhotoCard'
import { GalleryControls } from '@/components/gallery/GalleryControls'
import { EmptyState } from '@/components/gallery/EmptyState'
import { LoadingState } from '@/components/gallery/LoadingState'
import { Button } from '@/components/ui/Button'
import { usePhotos } from '@/context/PhotosContext'
import { AlertCircle, RefreshCw, CheckCircle2, FolderOpen } from 'lucide-react'

interface PhotosPageProps {
  onSelectPhoto: (photo: Photo, photosList: Photo[]) => void
  onUploadClick: () => void
  folderFilter?: string | null
  onClearFolderFilter?: () => void
}

export function PhotosPage({
  onSelectPhoto,
  onUploadClick,
  folderFilter,
  onClearFolderFilter,
}: PhotosPageProps) {
  const {
    photos,
    totalPhotos,
    isLoading,
    isLoadingMore,
    error,
    sortOption,
    setSortOption,
    viewMode,
    setViewMode,
    searchQuery,
    setSearchQuery,
    isScanning,
    scanResult,
    setScanResult,
    loadPhotos,
    loadMore,
    handleScan,
  } = usePhotos()

  // Filter photos client-side for search and folder
  const filteredPhotos = useMemo(() => {
    let result = [...photos]

    if (folderFilter) {
      result = result.filter((p) => p.folderId === folderFilter)
    }

    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase().trim()
      result = result.filter(
        (p) =>
          p.title.toLowerCase().includes(q) ||
          p.filename.toLowerCase().includes(q) ||
          (p.metadata.camera && p.metadata.camera.toLowerCase().includes(q))
      )
    }

    return result
  }, [photos, folderFilter, searchQuery])

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex flex-col sm:flex-row sm:items-end justify-between gap-2">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">
            {folderFilter ? 'Folder Photos' : 'All Photos'}
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            {folderFilter
              ? 'Photos inside the selected album.'
              : 'Browse, search, and curate your photographic collection.'}
          </p>
        </div>

        {folderFilter && onClearFolderFilter && (
          <button
            onClick={onClearFolderFilter}
            className="text-xs text-primary hover:underline font-medium self-start sm:self-auto"
          >
            &larr; View all photos
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
        totalCount={folderFilter ? filteredPhotos.length : totalPhotos}
        onScanClick={handleScan}
        isScanning={isScanning}
      />

      {/* Gallery Content Area */}
      {isLoading ? (
        <LoadingState viewMode={viewMode} />
      ) : folderFilter && filteredPhotos.length === 0 ? (
        <div className="flex flex-col items-center justify-center p-14 text-center border border-dashed border-border rounded-2xl bg-card/40 my-8">
          <div className="flex items-center justify-center w-14 h-14 rounded-full bg-primary/10 text-primary mb-4">
            <FolderOpen className="w-7 h-7 stroke-[1.5]" />
          </div>
          <h3 className="text-lg font-semibold text-foreground mb-1">No photos in this album</h3>
          <p className="text-sm text-muted-foreground max-w-md mb-5 leading-relaxed">
            This album has no assigned photos yet. Full album organization and assigning photos to albums is planned for Phase 4.
          </p>
          {onClearFolderFilter && (
            <Button onClick={onClearFolderFilter} size="md">
              View all photos
            </Button>
          )}
        </div>
      ) : filteredPhotos.length === 0 ? (
        <EmptyState
          isSearch={Boolean(searchQuery.trim())}
          searchQuery={searchQuery}
          onReset={() => setSearchQuery('')}
          onUploadClick={onUploadClick}
          onScanClick={handleScan}
          isScanning={isScanning}
        />
      ) : viewMode === 'grid' ? (
        <>
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 md:gap-6">
            {filteredPhotos.map((photo) => (
              <PhotoCard
                key={photo.id}
                photo={photo}
                onSelect={(p) => onSelectPhoto(p, filteredPhotos)}
                viewMode="grid"
              />
            ))}
          </div>

          {!folderFilter && photos.length < totalPhotos && (
            <div className="flex justify-center pt-6 pb-2">
              <Button
                variant="outline"
                size="md"
                onClick={loadMore}
                disabled={isLoadingMore}
                className="gap-2"
              >
                {isLoadingMore && <RefreshCw className="w-4 h-4 animate-spin text-primary" />}
                <span>
                  {isLoadingMore
                    ? 'Loading photos...'
                    : `Load More Photos (${photos.length} of ${totalPhotos})`}
                </span>
              </Button>
            </div>
          )}
        </>
      ) : (
        <>
          <div className="space-y-2.5">
            {filteredPhotos.map((photo) => (
              <PhotoCard
                key={photo.id}
                photo={photo}
                onSelect={(p) => onSelectPhoto(p, filteredPhotos)}
                viewMode="list"
              />
            ))}
          </div>

          {!folderFilter && photos.length < totalPhotos && (
            <div className="flex justify-center pt-6 pb-2">
              <Button
                variant="outline"
                size="md"
                onClick={loadMore}
                disabled={isLoadingMore}
                className="gap-2"
              >
                {isLoadingMore && <RefreshCw className="w-4 h-4 animate-spin text-primary" />}
                <span>
                  {isLoadingMore
                    ? 'Loading photos...'
                    : `Load More Photos (${photos.length} of ${totalPhotos})`}
                </span>
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
