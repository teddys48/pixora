import { useState, useEffect, useMemo, useCallback, useRef } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { FallbackImage } from '@/components/ui/FallbackImage'
import { Photo } from '@/types/photo'
import { Folder } from '@/types/folder'
import { fetchPhotos, addPhotosToAlbum, fetchAlbumPhotoIds } from '@/lib/api'
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

const PAGE_SIZE = 60

export function AddPhotosToAlbumModal({
  isOpen,
  album,
  currentPhotoIds,
  onClose,
  onAddSuccess,
}: AddPhotosToAlbumModalProps) {
  const [photos, setPhotos] = useState<Photo[]>([])
  const [totalPhotos, setTotalPhotos] = useState(0)
  const [hasMore, setHasMore] = useState(false)
  const [nextCursor, setNextCursor] = useState<string | null>(null)

  const [isLoading, setIsLoading] = useState(false)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [searchQuery, setSearchQuery] = useState('')
  const [debouncedSearchQuery, setDebouncedSearchQuery] = useState('')
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [albumPhotoIds, setAlbumPhotoIds] = useState<Set<string>>(new Set(currentPhotoIds))

  const scrollContainerRef = useRef<HTMLDivElement | null>(null)
  const abortControllerRef = useRef<AbortController | null>(null)

  // Debounce user search input
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearchQuery(searchQuery)
    }, 300)
    return () => clearTimeout(timer)
  }, [searchQuery])

  // Sync and fetch all photo IDs belonging to this album from backend to guarantee 100% accurate "In Album" detection
  useEffect(() => {
    if (!isOpen || !album?.id) return
    let active = true

    // Seed immediately with currentPhotoIds prop
    setAlbumPhotoIds(new Set(currentPhotoIds))

    fetchAlbumPhotoIds(album.id)
      .then((ids) => {
        if (active) {
          setAlbumPhotoIds(new Set(ids))
        }
      })
      .catch((err) => {
        console.warn('Could not fetch album photo IDs, using currentPhotoIds:', err)
      })

    return () => {
      active = false
    }
  }, [isOpen, album?.id, currentPhotoIds])

  // Load photos from library with server-side query and pagination
  const loadInitialPhotos = useCallback(async (query: string) => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
    }
    const controller = new AbortController()
    abortControllerRef.current = controller

    setIsLoading(true)
    setError(null)

    try {
      const res = await fetchPhotos(
        {
          limit: PAGE_SIZE,
          q: query.trim() || undefined,
        },
        controller.signal
      )

      setPhotos(res.photos)
      setTotalPhotos(res.total)
      setHasMore(Boolean(res.hasMore))
      setNextCursor(res.nextCursor ?? null)

      // Reset scroll position to top on new search / initial load
      if (scrollContainerRef.current) {
        scrollContainerRef.current.scrollTop = 0
      }
    } catch (err: unknown) {
      if (err instanceof DOMException && err.name === 'AbortError') return
      const msg = err instanceof Error ? err.message : 'Failed to load photos'
      setError(msg)
    } finally {
      setIsLoading(false)
    }
  }, [])

  // Load more photos when scrolling or clicking "Load more"
  const loadMorePhotos = useCallback(async () => {
    if (isLoadingMore || !hasMore || !nextCursor) return
    setIsLoadingMore(true)
    setError(null)

    try {
      const res = await fetchPhotos({
        limit: PAGE_SIZE,
        cursor: nextCursor,
        q: debouncedSearchQuery.trim() || undefined,
      })

      setPhotos((prev) => {
        const existing = new Set(prev.map((p) => p.id))
        const unique = res.photos.filter((p) => !existing.has(p.id))
        return [...prev, ...unique]
      })
      setTotalPhotos(res.total)
      setHasMore(Boolean(res.hasMore))
      setNextCursor(res.nextCursor ?? null)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load more photos'
      setError(msg)
    } finally {
      setIsLoadingMore(false)
    }
  }, [isLoadingMore, hasMore, nextCursor, debouncedSearchQuery])

  // Handle scroll inside photos grid container for seamless infinite scrolling
  const handleScroll = (e: React.UIEvent<HTMLDivElement>) => {
    const { scrollTop, scrollHeight, clientHeight } = e.currentTarget
    if (scrollHeight - scrollTop - clientHeight < 250 && hasMore && !isLoading && !isLoadingMore) {
      loadMorePhotos()
    }
  }

  // Reset state when modal opens
  useEffect(() => {
    if (isOpen) {
      setSelectedIds(new Set())
      setSearchQuery('')
      setDebouncedSearchQuery('')
      setError(null)
    }
  }, [isOpen])

  // Fetch photos whenever modal is open and search query stabilizes
  useEffect(() => {
    if (isOpen) {
      loadInitialPhotos(debouncedSearchQuery)
    }
  }, [isOpen, debouncedSearchQuery, loadInitialPhotos])

  // Helper to determine if a photo is in this album
  const isAlreadyInAlbum = useCallback(
    (photoId: string) => albumPhotoIds.has(photoId) || currentPhotoIds.has(photoId),
    [albumPhotoIds, currentPhotoIds]
  )

  // Count available photos currently rendered that are not already in the album
  const availablePhotos = useMemo(() => {
    return photos.filter((p) => !isAlreadyInAlbum(p.id))
  }, [photos, isAlreadyInAlbum])

  const toggleSelectPhoto = (photoId: string) => {
    if (isAlreadyInAlbum(photoId) || isSubmitting) return
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
    setSelectedIds((prev) => {
      const next = new Set(prev)
      availablePhotos.forEach((p) => next.add(p.id))
      return next
    })
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
      // Immediately register newly added IDs into albumPhotoIds
      setAlbumPhotoIds((prev) => {
        const next = new Set(prev)
        photoIds.forEach((id) => next.add(id))
        return next
      })
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
              placeholder="Search library photos by filename or title..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-9 h-9 text-xs"
              disabled={isSubmitting}
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
                  disabled={isLoading || isSubmitting}
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

        {/* Photos Grid with Infinite Scroll */}
        <div
          ref={scrollContainerRef}
          onScroll={handleScroll}
          className="relative min-h-[280px] max-h-[460px] overflow-y-auto rounded-xl border border-border/70 p-3 bg-muted/20"
        >
          {isLoading ? (
            <div className="flex flex-col items-center justify-center h-64 text-center">
              <Loader2 className="w-6 h-6 animate-spin text-primary mb-2" />
              <p className="text-xs text-muted-foreground">Loading library photos...</p>
            </div>
          ) : photos.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-64 text-center">
              <p className="text-sm font-medium text-foreground">No photos found</p>
              <p className="text-xs text-muted-foreground mt-1 max-w-xs">
                {debouncedSearchQuery
                  ? 'No photos matched your search term.'
                  : 'No indexed photos are available in the gallery. Use the Scan Media or Upload features to add photos.'}
              </p>
            </div>
          ) : (
            <>
              <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-3">
                {photos.map((photo) => {
                  const alreadyIn = isAlreadyInAlbum(photo.id)
                  const isSelected = selectedIds.has(photo.id)

                  return (
                    <div
                      key={photo.id}
                      onClick={() => toggleSelectPhoto(photo.id)}
                      className={`group relative aspect-square rounded-lg overflow-hidden border transition-all duration-200 select-none ${
                        alreadyIn
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
                      {alreadyIn && (
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

              {/* Infinite Scroll Footer / Load More Status */}
              <div className="pt-4 pb-2 flex flex-col items-center justify-center gap-2">
                {isLoadingMore ? (
                  <div className="flex items-center gap-2 text-xs text-muted-foreground py-2">
                    <Loader2 className="w-4 h-4 animate-spin text-primary" />
                    <span>Loading more photos...</span>
                  </div>
                ) : hasMore ? (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={loadMorePhotos}
                    className="h-8 text-xs font-normal"
                  >
                    Load More Photos ({photos.length} of {totalPhotos} loaded)
                  </Button>
                ) : totalPhotos > 0 ? (
                  <p className="text-[11px] text-muted-foreground/70 py-1">
                    All {totalPhotos} photo{totalPhotos === 1 ? '' : 's'} loaded
                  </p>
                ) : null}
              </div>
            </>
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
              <span>
                {totalPhotos > 0
                  ? `${photos.length} of ${totalPhotos} photos shown`
                  : 'Select photos to add to this album'}
              </span>
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
