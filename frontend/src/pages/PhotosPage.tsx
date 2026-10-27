import { useState, useMemo } from 'react'
import { Photo, PhotoSortOption, PhotoViewMode } from '@/types/photo'
import { MOCK_PHOTOS } from '@/data/mock/photos'
import { PhotoCard } from '@/components/gallery/PhotoCard'
import { GalleryControls } from '@/components/gallery/GalleryControls'
import { EmptyState } from '@/components/gallery/EmptyState'
import { LoadingState } from '@/components/gallery/LoadingState'

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
  const [searchQuery, setSearchQuery] = useState('')
  const [sortOption, setSortOption] = useState<PhotoSortOption>('date-desc')
  const [viewMode, setViewMode] = useState<PhotoViewMode>('grid')
  const [isLoading] = useState(false)

  // Filter and sort photos
  const filteredAndSortedPhotos = useMemo(() => {
    let result = [...MOCK_PHOTOS]

    // Apply folder filter if active
    if (folderFilter) {
      result = result.filter((p) => p.folderId === folderFilter)
    }

    // Apply search filter
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase().trim()
      result = result.filter(
        (p) =>
          p.title.toLowerCase().includes(q) ||
          p.filename.toLowerCase().includes(q) ||
          (p.metadata.camera && p.metadata.camera.toLowerCase().includes(q)) ||
          (p.tags && p.tags.some((t) => t.toLowerCase().includes(q)))
      )
    }

    // Apply sorting
    result.sort((a, b) => {
      switch (sortOption) {
        case 'date-desc':
          return new Date(b.date).getTime() - new Date(a.date).getTime()
        case 'date-asc':
          return new Date(a.date).getTime() - new Date(b.date).getTime()
        case 'title-asc':
          return a.title.localeCompare(b.title)
        case 'title-desc':
          return b.title.localeCompare(a.title)
        case 'size-desc':
          return b.size - a.size
        default:
          return 0
      }
    })

    return result
  }, [searchQuery, sortOption, folderFilter])

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

      {/* Controls Bar */}
      <GalleryControls
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        sortOption={sortOption}
        onSortChange={setSortOption}
        viewMode={viewMode}
        onViewModeChange={setViewMode}
        totalCount={filteredAndSortedPhotos.length}
      />

      {/* Gallery Content Area */}
      {isLoading ? (
        <LoadingState viewMode={viewMode} />
      ) : filteredAndSortedPhotos.length === 0 ? (
        <EmptyState
          isSearch={Boolean(searchQuery.trim())}
          searchQuery={searchQuery}
          onReset={() => setSearchQuery('')}
          onUploadClick={onUploadClick}
        />
      ) : viewMode === 'grid' ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 md:gap-6">
          {filteredAndSortedPhotos.map((photo) => (
            <PhotoCard
              key={photo.id}
              photo={photo}
              onSelect={(p) => onSelectPhoto(p, filteredAndSortedPhotos)}
              viewMode="grid"
            />
          ))}
        </div>
      ) : (
        <div className="space-y-2.5">
          {filteredAndSortedPhotos.map((photo) => (
            <PhotoCard
              key={photo.id}
              photo={photo}
              onSelect={(p) => onSelectPhoto(p, filteredAndSortedPhotos)}
              viewMode="list"
            />
          ))}
        </div>
      )}
    </div>
  )
}
