import { useState, useEffect, useCallback, useMemo } from 'react'
import { Folder } from '@/types/folder'
import { Photo, PhotoViewMode } from '@/types/photo'
import { PhotoCard } from '@/components/gallery/PhotoCard'
import { AddPhotosToAlbumModal } from '@/components/folders/AddPhotosToAlbumModal'
import { RenameFolderModal } from '@/components/folders/RenameFolderModal'
import { DeleteFolderModal } from '@/components/folders/DeleteFolderModal'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import {
  fetchAlbumPhotos,
  fetchAlbum,
  removePhotoFromAlbum,
  updateAlbum,
} from '@/lib/api'
import { formatDate } from '@/lib/utils'
import {
  ArrowLeft,
  FolderOpen,
  Plus,
  Edit2,
  Trash2,
  Search,
  Grid,
  List,
  RefreshCw,
  AlertCircle,
  CheckCircle2,
  Calendar,
  Images,
} from 'lucide-react'

interface AlbumDetailViewProps {
  album: Folder
  onBack: () => void
  onSelectPhoto: (photo: Photo, photosList: Photo[]) => void
  onUpdateAlbum: (updated: Folder) => void
  onDeleteAlbum: (albumId: string) => Promise<void> | void
}

export function AlbumDetailView({
  album,
  onBack,
  onSelectPhoto,
  onUpdateAlbum,
  onDeleteAlbum,
}: AlbumDetailViewProps) {
  const [photos, setPhotos] = useState<Photo[]>([])
  const [total, setTotal] = useState(album.photoCount)
  const [isLoading, setIsLoading] = useState(true)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [successBanner, setSuccessBanner] = useState<string | null>(null)

  const [searchQuery, setSearchQuery] = useState('')
  const [viewMode, setViewMode] = useState<PhotoViewMode>('grid')

  const [isAddOpen, setIsAddOpen] = useState(false)
  const [isRenameOpen, setIsRenameOpen] = useState(false)
  const [isDeleteOpen, setIsDeleteOpen] = useState(false)
  const [isRemovingId, setIsRemovingId] = useState<string | null>(null)

  const loadPhotos = useCallback(async () => {
    setIsLoading(true)
    setError(null)
    try {
      const res = await fetchAlbumPhotos(album.id, { limit: 50, offset: 0 })
      setPhotos(res.photos)
      setTotal(res.total)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load album photos'
      setError(msg)
    } finally {
      setIsLoading(false)
    }
  }, [album.id])

  useEffect(() => {
    loadPhotos()
  }, [loadPhotos])

  const loadMorePhotos = async () => {
    if (isLoadingMore || photos.length >= total) return
    setIsLoadingMore(true)
    try {
      const res = await fetchAlbumPhotos(album.id, { limit: 50, offset: photos.length })
      setPhotos((prev) => [...prev, ...res.photos])
      setTotal(res.total)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load more photos'
      setError(msg)
    } finally {
      setIsLoadingMore(false)
    }
  }

  const handleRemovePhoto = async (photo: Photo) => {
    if (isRemovingId) return
    setIsRemovingId(photo.id)
    setError(null)
    try {
      await removePhotoFromAlbum(album.id, photo.id)
      setPhotos((prev) => prev.filter((p) => p.id !== photo.id))
      const newTotal = Math.max(0, total - 1)
      setTotal(newTotal)
      onUpdateAlbum({ ...album, photoCount: newTotal })
      setSuccessBanner(`Removed "${photo.title}" from album. Original photo remains intact in your library.`)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to remove photo from album'
      setError(msg)
    } finally {
      setIsRemovingId(null)
    }
  }

  const handleAddPhotosSuccess = async (addedCount: number) => {
    setSuccessBanner(`Successfully added ${addedCount} photo${addedCount === 1 ? '' : 's'} to "${album.name}".`)
    await loadPhotos()
    try {
      const refreshedAlbum = await fetchAlbum(album.id)
      onUpdateAlbum(refreshedAlbum)
    } catch {
      // Ignore if background fetch fails
    }
  }

  const handleRename = async (id: string, newName: string, newDesc: string) => {
    const updated = await updateAlbum(id, newName, newDesc)
    onUpdateAlbum(updated)
    setSuccessBanner(`Album renamed to "${updated.name}".`)
  }

  const handleDelete = async (id: string) => {
    await onDeleteAlbum(id)
  }

  const filteredPhotos = useMemo(() => {
    if (!searchQuery.trim()) return photos
    const q = searchQuery.toLowerCase().trim()
    return photos.filter(
      (p) => p.title.toLowerCase().includes(q) || p.filename.toLowerCase().includes(q)
    )
  }, [photos, searchQuery])

  const currentPhotoIds = useMemo(() => {
    return new Set(photos.map((p) => p.id))
  }, [photos])

  return (
    <div className="space-y-6">
      {/* Navigation & Header */}
      <div className="space-y-3">
        <button
          type="button"
          onClick={onBack}
          className="inline-flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground font-medium transition-colors"
        >
          <ArrowLeft className="w-3.5 h-3.5" />
          <span>Back to Albums</span>
        </button>

        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pt-1">
          <div className="space-y-1">
            <div className="flex items-center gap-2.5">
              <h1 className="text-2xl font-bold tracking-tight text-foreground truncate max-w-md sm:max-w-xl">
                {album.name}
              </h1>
              <span className="text-xs font-medium text-muted-foreground bg-muted px-2.5 py-0.5 rounded-full border border-border/60 shrink-0">
                {total} photo{total === 1 ? '' : 's'}
              </span>
            </div>
            {album.description && (
              <p className="text-sm text-muted-foreground max-w-2xl leading-relaxed">
                {album.description}
              </p>
            )}
            <div className="flex items-center gap-3 text-[11px] text-muted-foreground pt-0.5">
              <span className="flex items-center gap-1">
                <Calendar className="w-3 h-3" />
                Updated {formatDate(album.updatedAt)}
              </span>
            </div>
          </div>

          <div className="flex items-center gap-2 self-start sm:self-auto shrink-0">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => setIsRenameOpen(true)}
              className="gap-1.5 h-9 text-xs"
            >
              <Edit2 className="w-3.5 h-3.5" />
              <span>Rename</span>
            </Button>

            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => setIsDeleteOpen(true)}
              className="gap-1.5 h-9 text-xs text-destructive hover:bg-destructive/10 hover:text-destructive border-destructive/20"
            >
              <Trash2 className="w-3.5 h-3.5" />
              <span>Delete</span>
            </Button>

            <Button
              type="button"
              size="sm"
              onClick={() => setIsAddOpen(true)}
              className="gap-1.5 h-9 text-xs font-medium"
            >
              <Plus className="w-4 h-4" />
              <span>Add Photos</span>
            </Button>
          </div>
        </div>
      </div>

      {/* Success Notification Banner */}
      {successBanner && (
        <div className="flex items-center justify-between p-3 rounded-xl border border-emerald-500/20 bg-emerald-500/10 text-xs text-foreground animate-in fade-in duration-200">
          <div className="flex items-center gap-2">
            <CheckCircle2 className="w-4 h-4 text-emerald-500 shrink-0" />
            <span>{successBanner}</span>
          </div>
          <button
            type="button"
            onClick={() => setSuccessBanner(null)}
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
          <Button variant="outline" size="sm" onClick={loadPhotos} className="h-7 text-xs ml-3">
            Retry
          </Button>
        </div>
      )}

      {/* Filter and View Controls Bar */}
      {photos.length > 0 && (
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 p-2 rounded-xl bg-card border border-border/70 text-xs">
          <div className="relative flex-1 max-w-sm">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground" />
            <Input
              type="text"
              placeholder="Search in this album..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-8 h-8 text-xs border-transparent bg-transparent focus-visible:bg-background"
            />
          </div>

          <div className="flex items-center gap-2 self-end sm:self-auto">
            <span className="text-muted-foreground text-[11px] pr-2">
              Showing {filteredPhotos.length} of {total}
            </span>
            <div className="flex items-center rounded-lg border border-border p-0.5 bg-muted/30">
              <button
                type="button"
                onClick={() => setViewMode('grid')}
                className={`p-1.5 rounded-md transition-colors ${
                  viewMode === 'grid'
                    ? 'bg-background shadow-xs text-foreground'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
                aria-label="Grid view"
              >
                <Grid className="w-3.5 h-3.5" />
              </button>
              <button
                type="button"
                onClick={() => setViewMode('list')}
                className={`p-1.5 rounded-md transition-colors ${
                  viewMode === 'list'
                    ? 'bg-background shadow-xs text-foreground'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
                aria-label="List view"
              >
                <List className="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Content Area */}
      {isLoading ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 md:gap-6 animate-pulse">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="aspect-[4/3] rounded-xl bg-muted/40 border border-border/60" />
          ))}
        </div>
      ) : photos.length === 0 ? (
        <div className="flex flex-col items-center justify-center p-14 text-center border border-dashed border-border rounded-2xl bg-card/40 my-8">
          <div className="flex items-center justify-center w-14 h-14 rounded-full bg-primary/10 text-primary mb-4">
            <FolderOpen className="w-7 h-7 stroke-[1.5]" />
          </div>
          <h3 className="text-lg font-semibold text-foreground mb-1">No photos in this album yet</h3>
          <p className="text-sm text-muted-foreground max-w-md mb-5 leading-relaxed">
            This virtual album has no assigned photos. You can easily add indexed photos from your Pixora library without copying or modifying original files.
          </p>
          <Button onClick={() => setIsAddOpen(true)} size="md" className="gap-2">
            <Plus className="w-4 h-4" />
            <span>Add Photos to Album</span>
          </Button>
        </div>
      ) : filteredPhotos.length === 0 ? (
        <div className="flex flex-col items-center justify-center p-12 text-center border border-dashed border-border rounded-2xl bg-card/40">
          <p className="text-sm font-medium text-foreground mb-1">No matching photos found</p>
          <p className="text-xs text-muted-foreground mb-4">
            No photos in this album matched &quot;{searchQuery}&quot;.
          </p>
          <Button variant="outline" size="sm" onClick={() => setSearchQuery('')}>
            Clear Search Filter
          </Button>
        </div>
      ) : viewMode === 'grid' ? (
        <>
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 md:gap-6">
            {filteredPhotos.map((photo) => (
              <PhotoCard
                key={photo.id}
                photo={photo}
                onSelect={(p) => onSelectPhoto(p, filteredPhotos)}
                viewMode="grid"
                onRemove={handleRemovePhoto}
                removeLabel="Remove from album"
              />
            ))}
          </div>

          {photos.length < total && (
            <div className="flex justify-center pt-6 pb-2">
              <Button
                variant="outline"
                size="md"
                onClick={loadMorePhotos}
                disabled={isLoadingMore}
                className="gap-2 text-xs"
              >
                {isLoadingMore ? (
                  <RefreshCw className="w-4 h-4 animate-spin text-primary" />
                ) : (
                  <Images className="w-4 h-4 text-muted-foreground" />
                )}
                <span>
                  {isLoadingMore
                    ? 'Loading photos...'
                    : `Load More Photos (${photos.length} of ${total})`}
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
                onRemove={handleRemovePhoto}
                removeLabel="Remove from album"
              />
            ))}
          </div>

          {photos.length < total && (
            <div className="flex justify-center pt-6 pb-2">
              <Button
                variant="outline"
                size="md"
                onClick={loadMorePhotos}
                disabled={isLoadingMore}
                className="gap-2 text-xs"
              >
                {isLoadingMore ? (
                  <RefreshCw className="w-4 h-4 animate-spin text-primary" />
                ) : (
                  <Images className="w-4 h-4 text-muted-foreground" />
                )}
                <span>
                  {isLoadingMore
                    ? 'Loading photos...'
                    : `Load More Photos (${photos.length} of ${total})`}
                </span>
              </Button>
            </div>
          )}
        </>
      )}

      {/* Add Photos Modal */}
      <AddPhotosToAlbumModal
        isOpen={isAddOpen}
        album={album}
        currentPhotoIds={currentPhotoIds}
        onClose={() => setIsAddOpen(false)}
        onAddSuccess={handleAddPhotosSuccess}
      />

      {/* Rename Album Modal */}
      <RenameFolderModal
        isOpen={isRenameOpen}
        folder={album}
        onClose={() => setIsRenameOpen(false)}
        onRename={handleRename}
      />

      {/* Delete Album Modal */}
      <DeleteFolderModal
        isOpen={isDeleteOpen}
        folder={album}
        onClose={() => setIsDeleteOpen(false)}
        onConfirm={handleDelete}
      />
    </div>
  )
}
