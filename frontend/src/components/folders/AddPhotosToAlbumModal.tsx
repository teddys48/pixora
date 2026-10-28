import { useState, useEffect, useMemo, useCallback } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { FallbackImage } from '@/components/ui/FallbackImage'
import { Photo } from '@/types/photo'
import { Folder } from '@/types/folder'
import { fetchPhotos, addPhotosToAlbum } from '@/lib/api'
import {
  Check,
  Search,
  CheckCircle2,
  AlertCircle,
  Loader2,
  Plus,
} from 'lucide-react'

interface AddPhotosToAlbumModalProps {
  isOpen: boolean
  album: Folder | null
  currentPhotoIds: Set<string>
  onClose: () => void
  onAddSuccess: (addedCount: number) => void
}

export function AddPhotosToAlbumModal({
  isOpen,
  album,
  currentPhotoIds,
  onClose,
  onAddSuccess,
}: AddPhotosToAlbumModalProps) {
  const [photos, setPhotos] = useState<Photo[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())

  const loadLibraryPhotos = useCallback(async () => {
    setIsLoading(true)
    setError(null)
    try {
      const res = await fetchPhotos({ limit: 100 })
      setPhotos(res.photos)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load photos'
      setError(msg)
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    if (isOpen) {
      setSelectedIds(new Set())
      setSearchQuery('')
      setError(null)
      loadLibraryPhotos()
    }
  }, [isOpen, loadLibraryPhotos])

  const filteredPhotos = useMemo(() => {
    if (!searchQuery.trim()) return photos
    const q = searchQuery.toLowerCase().trim()
    return photos.filter(
      (p) => p.title.toLowerCase().includes(q) || p.filename.toLowerCase().includes(q)
    )
  }, [photos, searchQuery])

  // Count available photos that are not already in the album
  const availablePhotos = useMemo(() => {
    return filteredPhotos.filter((p) => !currentPhotoIds.has(p.id))
  }, [filteredPhotos, currentPhotoIds])

  const toggleSelectPhoto = (photoId: string) => {
    if (currentPhotoIds.has(photoId) || isSubmitting) return
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(photoId)) {
        next.delete(photoId)
      } else {
        next.add(photoId)
      }
      return next
    })
  }

  const handleSelectAll = () => {
    const allAvailableIds = availablePhotos.map((p) => p.id)
    setSelectedIds(new Set(allAvailableIds))
  }

  const handleDeselectAll = () => {
    setSelectedIds(new Set())
  }

  const handleSubmit = async () => {
    if (!album || selectedIds.size === 0 || isSubmitting) return
    setIsSubmitting(true)
    setError(null)
    try {
      const photoIds = Array.from(selectedIds)
      const res = await addPhotosToAlbum(album.id, photoIds)
      onAddSuccess(res.added)
      onClose()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to add photos to album'
      setError(msg)
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={() => {
        if (!isSubmitting) onClose()
      }}
      title={`Add Photos to "${album?.name || 'Album'}"`}
      description="Choose photos from your library to add to this album. Original files will not be moved or copied."
      maxWidth="xl"
    >
      <div className="space-y-4">
        {/* Error Alert */}
        {error && (
          <div className="flex items-center gap-2 p-3 rounded-lg bg-destructive/10 border border-destructive/20 text-destructive text-xs">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {/* Search & Bulk Select Controls */}
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 pt-1">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <Input
              type="text"
              placeholder="Search library photos..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-9 h-9 text-xs"
              disabled={isLoading || isSubmitting}
            />
          </div>

          <div className="flex items-center gap-2 text-xs">
            {availablePhotos.length > 0 && (
              <>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={handleSelectAll}
                  disabled={isLoading || isSubmitting || selectedIds.size === availablePhotos.length}
                  className="h-8 text-xs font-normal"
                >
                  Select All ({availablePhotos.length})
                </Button>
                {selectedIds.size > 0 && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={handleDeselectAll}
                    disabled={isSubmitting}
                    className="h-8 text-xs text-muted-foreground hover:text-foreground"
                  >
                    Clear Selection
                  </Button>
                )}
              </>
            )}
          </div>
        </div>

        {/* Photos Grid */}
        <div className="relative min-h-[280px] max-h-[460px] overflow-y-auto rounded-xl border border-border/70 p-3 bg-muted/20">
          {isLoading ? (
            <div className="flex flex-col items-center justify-center h-64 text-center">
              <Loader2 className="w-6 h-6 animate-spin text-primary mb-2" />
              <p className="text-xs text-muted-foreground">Loading library photos...</p>
            </div>
          ) : filteredPhotos.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-64 text-center">
              <p className="text-sm font-medium text-foreground">No photos found</p>
              <p className="text-xs text-muted-foreground mt-1 max-w-xs">
                {searchQuery
                  ? 'No photos matched your search term.'
                  : 'No indexed photos are available in the gallery. Use the Scan Media or Upload features to add photos.'}
              </p>
            </div>
          ) : (
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-3">
              {filteredPhotos.map((photo) => {
                const isAlreadyInAlbum = currentPhotoIds.has(photo.id)
                const isSelected = selectedIds.has(photo.id)

                return (
                  <div
                    key={photo.id}
                    onClick={() => toggleSelectPhoto(photo.id)}
                    className={`group relative aspect-square rounded-lg overflow-hidden border transition-all duration-200 select-none ${
                      isAlreadyInAlbum
                        ? 'opacity-60 border-border/50 cursor-not-allowed bg-muted/50'
                        : isSelected
                        ? 'border-primary ring-2 ring-primary/30 shadow-md cursor-pointer'
                        : 'border-border/70 hover:border-foreground/40 cursor-pointer bg-card'
                    }`}
                  >
                    <FallbackImage
                      src={photo.thumbnailUrl}
                      alt={photo.title}
                      className="w-full h-full object-cover transition-transform duration-300 group-hover:scale-105"
                    />

                    {/* Selection Overlay */}
                    {isSelected && (
                      <div className="absolute inset-0 bg-primary/20 backdrop-blur-[1px] flex items-center justify-center">
                        <div className="w-7 h-7 rounded-full bg-primary text-primary-foreground flex items-center justify-center shadow-lg animate-in zoom-in-75 duration-150">
                          <Check className="w-4 h-4 stroke-[3]" />
                        </div>
                      </div>
                    )}

                    {/* Already In Album Badge */}
                    {isAlreadyInAlbum && (
                      <div className="absolute inset-0 bg-black/40 flex items-center justify-center p-2 text-center">
                        <span className="flex items-center gap-1 text-[11px] font-medium text-white px-2 py-0.5 rounded-full bg-black/70 backdrop-blur-sm border border-white/20">
                          <CheckCircle2 className="w-3 h-3 text-emerald-400" />
                          In Album
                        </span>
                      </div>
                    )}

                    {/* Filename hover label */}
                    <div className="absolute bottom-0 inset-x-0 bg-gradient-to-t from-black/80 to-transparent p-1.5 opacity-0 group-hover:opacity-100 transition-opacity">
                      <p className="text-[10px] text-white truncate">{photo.title}</p>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>

        {/* Modal Footer */}
        <div className="flex items-center justify-between pt-3 border-t border-border">
          <div className="text-xs text-muted-foreground">
            {selectedIds.size > 0 ? (
              <span className="font-medium text-foreground">
                {selectedIds.size} photo{selectedIds.size === 1 ? '' : 's'} selected
              </span>
            ) : (
              <span>Select photos to add to this album</span>
            )}
          </div>

          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={onClose}
              disabled={isSubmitting}
            >
              Cancel
            </Button>
            <Button
              type="button"
              size="sm"
              onClick={handleSubmit}
              disabled={selectedIds.size === 0 || isSubmitting}
              className="gap-1.5"
            >
              {isSubmitting ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <Plus className="w-4 h-4" />
              )}
              <span>
                {isSubmitting
                  ? 'Adding...'
                  : `Add ${selectedIds.size > 0 ? selectedIds.size : ''} Photo${
                      selectedIds.size === 1 ? '' : 's'
                    }`}
              </span>
            </Button>
          </div>
        </div>
      </div>
    </Modal>
  )
}
