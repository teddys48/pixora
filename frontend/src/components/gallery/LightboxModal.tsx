import React, { useEffect, useState, useCallback, useRef } from 'react'
import { Photo } from '@/types/photo'
import { formatDate, formatBytes } from '@/lib/utils'
import {
  X,
  ChevronLeft,
  ChevronRight,
  Info,
  Calendar,
  Camera,
  Maximize,
  HardDrive,
  Aperture,
  Tag,
  ZoomIn,
  ZoomOut,
  RotateCcw,
  ImageOff,
  Folder as FolderIcon,
  Loader2,
  Clock,
} from 'lucide-react'
import { cn } from '@/lib/utils'

interface LightboxModalProps {
  photo: Photo | null
  photos: Photo[]
  isOpen: boolean
  onClose: () => void
  onNavigate: (photo: Photo) => void
  hasMore?: boolean
  onLoadMore?: () => void
  isLoadingMore?: boolean
}

export function LightboxModal({
  photo,
  photos,
  isOpen,
  onClose,
  onNavigate,
  hasMore = false,
  onLoadMore,
  isLoadingMore = false,
}: LightboxModalProps) {
  const [showInfo, setShowInfo] = useState(false)
  const [zoomScale, setZoomScale] = useState(1)
  const [panOffset, setPanOffset] = useState({ x: 0, y: 0 })
  const [isPanning, setIsPanning] = useState(false)
  const [isImgLoading, setIsImgLoading] = useState(true)
  const [imgHasError, setImgHasError] = useState(false)

  const panStartRef = useRef({ x: 0, y: 0 })
  const currentOffsetRef = useRef({ x: 0, y: 0 })

  const currentIndex = photo ? photos.findIndex((p) => p.id === photo.id) : -1
  const hasPrev = currentIndex > 0
  const hasNext = currentIndex >= 0 && (currentIndex < photos.length - 1 || Boolean(hasMore))

  const pendingNextRef = useRef(false)

  // Reset zoom & pan when switching photos or closing
  const resetZoom = useCallback(() => {
    setZoomScale(1)
    setPanOffset({ x: 0, y: 0 })
    currentOffsetRef.current = { x: 0, y: 0 }
  }, [])

  useEffect(() => {
    resetZoom()
    setIsImgLoading(true)
    setImgHasError(false)
  }, [photo?.id, resetZoom])

  // Speculatively load next page when approaching end of collection
  useEffect(() => {
    if (currentIndex >= photos.length - 2 && hasMore && onLoadMore && !isLoadingMore) {
      onLoadMore()
    }
  }, [currentIndex, photos.length, hasMore, onLoadMore, isLoadingMore])

  // Advance photo if user was waiting at collection boundary for next page
  useEffect(() => {
    if (pendingNextRef.current && currentIndex >= 0 && currentIndex < photos.length - 1) {
      pendingNextRef.current = false
      onNavigate(photos[currentIndex + 1])
    }
  }, [photos, currentIndex, onNavigate])

  const handlePrev = useCallback(() => {
    if (hasPrev) {
      onNavigate(photos[currentIndex - 1])
    }
  }, [hasPrev, photos, currentIndex, onNavigate])

  const handleNext = useCallback(() => {
    if (currentIndex >= 0 && currentIndex < photos.length - 1) {
      onNavigate(photos[currentIndex + 1])
    } else if (hasMore && onLoadMore) {
      pendingNextRef.current = true
      onLoadMore()
    }
  }, [currentIndex, photos, hasMore, onLoadMore, onNavigate])

  const handleClose = useCallback(() => {
    handleClose()
    if (photo?.id) {
      setTimeout(() => {
        const el = document.getElementById(`photo-${photo.id}`)
        el?.scrollIntoView({ block: "nearest" })
      }, 50)
    }
  }, [onClose, photo?.id])

  const handleZoomIn = useCallback(() => {
    setZoomScale((prev) => Math.min(prev + 0.5, 4))
  }, [])

  const handleZoomOut = useCallback(() => {
    setZoomScale((prev) => {
      const next = Math.max(prev - 0.5, 1)
      if (next === 1) {
        setPanOffset({ x: 0, y: 0 })
        currentOffsetRef.current = { x: 0, y: 0 }
      }
      return next
    })
  }, [])

  // Fit to width: set zoom scale to 1.5 as comfortable reading/width fit
  const handleFitToWidth = useCallback(() => {
    setZoomScale(1.75)
    setPanOffset({ x: 0, y: 0 })
    currentOffsetRef.current = { x: 0, y: 0 }
  }, [])

  // Mouse pan handlers
  const handleMouseDown = (e: React.MouseEvent) => {
    if (zoomScale <= 1) return
    setIsPanning(true)
    panStartRef.current = {
      x: e.clientX - panOffset.x,
      y: e.clientY - panOffset.y,
    }
  }

  const handleMouseMove = (e: React.MouseEvent) => {
    if (!isPanning || zoomScale <= 1) return
    const newX = e.clientX - panStartRef.current.x
    const newY = e.clientY - panStartRef.current.y
    setPanOffset({ x: newX, y: newY })
    currentOffsetRef.current = { x: newX, y: newY }
  }

  const handleMouseUp = () => {
    setIsPanning(false)
  }

  // Touch pan handlers
  const handleTouchStart = (e: React.TouchEvent) => {
    if (zoomScale <= 1 || e.touches.length !== 1) return
    setIsPanning(true)
    panStartRef.current = {
      x: e.touches[0].clientX - panOffset.x,
      y: e.touches[0].clientY - panOffset.y,
    }
  }

  const handleTouchMove = (e: React.TouchEvent) => {
    if (!isPanning || zoomScale <= 1 || e.touches.length !== 1) return
    const newX = e.touches[0].clientX - panStartRef.current.x
    const newY = e.touches[0].clientY - panStartRef.current.y
    setPanOffset({ x: newX, y: newY })
    currentOffsetRef.current = { x: newX, y: newY }
  }

  const handleTouchEnd = () => {
    setIsPanning(false)
  }

  // Keyboard navigation with input field safeguard
  useEffect(() => {
    if (!isOpen) return

    const originalOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'

    const handleKeyDown = (e: KeyboardEvent) => {
      // Prevent keyboard shortcuts from triggering when user is typing in an input
      const target = e.target as HTMLElement | null
      const tagName = target?.tagName?.toLowerCase()
      if (
        tagName === 'input' ||
        tagName === 'textarea' ||
        tagName === 'select' ||
        target?.isContentEditable
      ) {
        return
      }

      if (e.key === 'Escape') {
        handleClose()
      } else if (e.key === 'ArrowLeft') {
        handlePrev()
      } else if (e.key === 'ArrowRight') {
        handleNext()
      } else if (e.key.toLowerCase() === 'i') {
        setShowInfo((prev) => !prev)
      } else if (e.key === '+' || e.key === '=') {
        handleZoomIn()
      } else if (e.key === '-' || e.key === '_') {
        handleZoomOut()
      } else if (e.key === '0') {
        resetZoom()
      }
    }

    window.addEventListener('keydown', handleKeyDown)

    return () => {
      document.body.style.overflow = originalOverflow
      window.removeEventListener('keydown', handleKeyDown)
    }
  }, [isOpen, onClose, handlePrev, handleNext, handleZoomIn, handleZoomOut, resetZoom])

  if (!isOpen || !photo) return null

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={`Photo viewer: ${photo.title}`}
      className="fixed inset-0 z-50 flex flex-col bg-black/95 text-white backdrop-blur-md select-none"
    >
      {/* Top Header Bar */}
      <div className="flex h-14 items-center justify-between px-4 border-b border-white/10 z-20 shrink-0 bg-black/40">
        <div className="flex items-center gap-3 min-w-0">
          <span className="text-xs text-neutral-400 font-mono">
            {currentIndex >= 0 ? `${currentIndex + 1} / ${photos.length}` : photo.filename}
          </span>
          <span className="text-sm font-medium text-neutral-200 truncate max-w-xs md:max-w-md">
            {photo.title}
          </span>
        </div>

        {/* Viewer Action Controls */}
        <div className="flex items-center gap-1 sm:gap-2">
          {/* Zoom controls */}
          <div className="flex items-center bg-white/10 rounded-lg p-0.5 border border-white/10">
            <button
              onClick={handleZoomOut}
              disabled={zoomScale <= 1}
              aria-label="Zoom out (-)"
              title="Zoom out (-)"
              className="p-1.5 rounded text-neutral-300 hover:text-white hover:bg-white/10 disabled:opacity-30 disabled:pointer-events-none transition-colors"
            >
              <ZoomOut className="h-4 w-4" />
            </button>
            <span className="text-[11px] font-mono px-2 text-neutral-300 min-w-[42px] text-center">
              {Math.round(zoomScale * 100)}%
            </span>
            <button
              onClick={handleZoomIn}
              disabled={zoomScale >= 4}
              aria-label="Zoom in (+)"
              title="Zoom in (+)"
              className="p-1.5 rounded text-neutral-300 hover:text-white hover:bg-white/10 disabled:opacity-30 disabled:pointer-events-none transition-colors"
            >
              <ZoomIn className="h-4 w-4" />
            </button>
          </div>

          {/* Reset Zoom / Fit Screen */}
          {zoomScale > 1 && (
            <button
              onClick={resetZoom}
              aria-label="Fit to screen (0)"
              title="Fit to screen (0)"
              className="p-2 rounded-lg text-neutral-300 hover:text-white hover:bg-white/10 transition-colors"
            >
              <RotateCcw className="h-4 w-4" />
            </button>
          )}

          {/* Fit to width */}
          <button
            onClick={zoomScale === 1 ? handleFitToWidth : resetZoom}
            aria-label={zoomScale === 1 ? 'Fit to width' : 'Fit to screen'}
            title={zoomScale === 1 ? 'Fit to width' : 'Fit to screen'}
            className="p-2 rounded-lg text-neutral-300 hover:text-white hover:bg-white/10 transition-colors hidden sm:flex"
          >
            <Maximize className="h-4 w-4" />
          </button>

          {/* Info toggle */}
          <button
            onClick={() => setShowInfo((prev) => !prev)}
            aria-label="Toggle photo metadata"
            aria-pressed={showInfo}
            className={cn(
              'p-2 rounded-lg text-neutral-400 hover:text-white transition-colors focus:outline-none focus-visible:ring-1 focus-visible:ring-white',
              showInfo ? 'bg-white/15 text-white' : 'hover:bg-white/10'
            )}
            title="Toggle Info (I)"
          >
            <Info className="h-4 w-4" />
          </button>

          {/* Close button */}
          <button
            onClick={handleClose}
            aria-label="Close viewer"
            className="p-2 rounded-lg text-neutral-400 hover:text-white hover:bg-white/10 transition-colors focus:outline-none focus-visible:ring-1 focus-visible:ring-white"
            title="Close (Esc)"
          >
            <X className="h-5 w-5" />
          </button>
        </div>
      </div>

      {/* Main View Area */}
      <div className="relative flex flex-1 overflow-hidden">
        {/* Left Nav Button */}
        {hasPrev && (
          <button
            onClick={handlePrev}
            aria-label="Previous photo"
            className="absolute left-4 top-1/2 -translate-y-1/2 z-30 flex h-11 w-11 items-center justify-center rounded-full bg-black/60 text-white/80 hover:text-white hover:bg-black/90 backdrop-blur-sm border border-white/10 transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-white"
            title="Previous (Arrow Left)"
          >
            <ChevronLeft className="h-6 w-6" />
          </button>
        )}

        {/* Right Nav Button */}
        {hasNext && (
          <button
            onClick={handleNext}
            aria-label="Next photo"
            className="absolute right-4 top-1/2 -translate-y-1/2 z-30 flex h-11 w-11 items-center justify-center rounded-full bg-black/60 text-white/80 hover:text-white hover:bg-black/90 backdrop-blur-sm border border-white/10 transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-white"
            title="Next (Arrow Right)"
          >
            {isLoadingMore && currentIndex === photos.length - 1 ? (
              <Loader2 className="h-5 w-5 animate-spin" />
            ) : (
              <ChevronRight className="h-6 w-6" />
            )}
          </button>
        )}

        {/* Center Image Display Area */}
        <div
          className={cn(
            'flex-1 flex items-center justify-center p-2 sm:p-6 md:p-8 overflow-hidden relative',
            zoomScale > 1
              ? isPanning
                ? 'cursor-grabbing'
                : 'cursor-grab'
              : 'cursor-default'
          )}
          onMouseDown={handleMouseDown}
          onMouseMove={handleMouseMove}
          onMouseUp={handleMouseUp}
          onMouseLeave={handleMouseUp}
          onTouchStart={handleTouchStart}
          onTouchMove={handleTouchMove}
          onTouchEnd={handleTouchEnd}
        >
          {/* Loading Indicator */}
          {isImgLoading && !imgHasError && (
            <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 z-10 pointer-events-none">
              <Loader2 className="w-8 h-8 text-white/70 animate-spin" />
              <span className="text-xs text-white/60 font-medium tracking-wide">
                Loading original photo...
              </span>
            </div>
          )}

          {/* Error State */}
          {imgHasError ? (
            <div className="flex flex-col items-center justify-center bg-white/5 border border-white/10 text-neutral-300 p-8 text-center rounded-2xl max-w-sm">
              <ImageOff className="w-12 h-12 mb-3 text-neutral-400 stroke-[1.5]" />
              <h4 className="text-base font-semibold text-white mb-1">{photo.title}</h4>
              <p className="text-xs text-neutral-400 mb-4 font-mono break-all">{photo.filename}</p>
              <span className="text-xs px-2.5 py-1 rounded bg-destructive/20 text-destructive-foreground border border-destructive/30">
                Failed to load original image file
              </span>
            </div>
          ) : (
            <div
              style={{
                transform: `translate(${panOffset.x}px, ${panOffset.y}px) scale(${zoomScale})`,
                transition: isPanning ? 'none' : 'transform 150ms ease-out',
              }}
              className="relative max-h-full max-w-full flex items-center justify-center origin-center"
            >
              <img
                src={photo.url}
                alt={photo.title}
                draggable={false}
                onLoad={() => setIsImgLoading(false)}
                onError={() => {
                  setIsImgLoading(false)
                  setImgHasError(true)
                }}
                className={cn(
                  'max-h-[82vh] max-w-[88vw] w-auto h-auto object-contain rounded-lg shadow-2xl transition-opacity duration-200 select-none pointer-events-none',
                  isImgLoading ? 'opacity-0' : 'opacity-100'
                )}
              />
            </div>
          )}
        </div>

        {/* Metadata Sidebar Panel (Desktop / Tablet) */}
        {showInfo && (
          <aside className="w-80 shrink-0 border-l border-white/10 bg-neutral-900/95 backdrop-blur-md p-6 overflow-y-auto space-y-6 hidden md:block animate-in slide-in-from-right-4 duration-200 z-20">
            <div>
              <h2 className="text-base font-semibold text-white tracking-tight">{photo.title}</h2>
              <p className="text-xs text-neutral-400 font-mono mt-1 break-all">{photo.filename}</p>
            </div>

            {/* Photo Details List */}
            <div className="space-y-4 text-xs">
              {/* Capture Date & Time */}
              <div className="flex items-start gap-3">
                <Calendar className="w-4 h-4 text-neutral-400 shrink-0 mt-0.5" />
                <div>
                  <div className="flex items-center gap-1.5">
                    <p className="text-neutral-400 font-medium">Capture Date</p>
                    {photo.takenAt ? (
                      <span className="text-[10px] px-1.5 py-0.2 rounded bg-primary/20 text-primary border border-primary/30">
                        EXIF
                      </span>
                    ) : (
                      <span className="text-[10px] px-1.5 py-0.2 rounded bg-muted/30 text-neutral-400">
                        Undated
                      </span>
                    )}
                  </div>
                  {photo.takenAt ? (
                    <p className="text-neutral-200 mt-0.5">{formatDate(photo.takenAt)}</p>
                  ) : (
                    <p className="text-neutral-400 mt-0.5 italic">
                      No capture date available
                    </p>
                  )}
                </div>
              </div>

              {/* File Modification Date (Fallback Context) */}
              <div className="flex items-start gap-3">
                <Clock className="w-4 h-4 text-neutral-400 shrink-0 mt-0.5" />
                <div>
                  <p className="text-neutral-400 font-medium">File Modified (Fallback)</p>
                  <p className="text-neutral-200 mt-0.5">
                    {photo.fileModifiedAt ? formatDate(photo.fileModifiedAt) : 'Unavailable'}
                  </p>
                </div>
              </div>

              {/* Physical Relative Folder */}
              <div className="flex items-start gap-3">
                <FolderIcon className="w-4 h-4 text-neutral-400 shrink-0 mt-0.5" />
                <div>
                  <p className="text-neutral-400 font-medium">Physical Folder</p>
                  <p className="text-neutral-200 font-mono text-[11px] mt-0.5 break-all">
                    {photo.folder ? photo.folder : 'Root (/media)'}
                  </p>
                </div>
              </div>

              {/* Dimensions */}
              <div className="flex items-start gap-3">
                <Maximize className="w-4 h-4 text-neutral-400 shrink-0 mt-0.5" />
                <div>
                  <p className="text-neutral-400 font-medium">Dimensions</p>
                  <p className="text-neutral-200 font-mono mt-0.5">
                    {photo.metadata.width > 0 && photo.metadata.height > 0
                      ? `${photo.metadata.width} × ${photo.metadata.height} px`
                      : 'Unavailable'}
                  </p>
                </div>
              </div>

              {/* File Size */}
              <div className="flex items-start gap-3">
                <HardDrive className="w-4 h-4 text-neutral-400 shrink-0 mt-0.5" />
                <div>
                  <p className="text-neutral-400 font-medium">File Size</p>
                  <p className="text-neutral-200 font-mono mt-0.5">{formatBytes(photo.size)}</p>
                </div>
              </div>

              {/* Camera & Lens */}
              {(photo.metadata.camera || photo.metadata.lens) && (
                <div className="flex items-start gap-3 pt-2 border-t border-white/10">
                  <Camera className="w-4 h-4 text-neutral-400 shrink-0 mt-0.5" />
                  <div>
                    <p className="text-neutral-400 font-medium">Camera</p>
                    {photo.metadata.camera && (
                      <p className="text-neutral-200 mt-0.5">{photo.metadata.camera}</p>
                    )}
                    {photo.metadata.lens && (
                      <p className="text-neutral-400 text-[11px] mt-0.5">{photo.metadata.lens}</p>
                    )}
                  </div>
                </div>
              )}

              {/* Exposure Settings */}
              {(photo.metadata.aperture ||
                photo.metadata.shutterSpeed ||
                photo.metadata.iso ||
                photo.metadata.focalLength) && (
                <div className="flex items-start gap-3">
                  <Aperture className="w-4 h-4 text-neutral-400 shrink-0 mt-0.5" />
                  <div>
                    <p className="text-neutral-400 font-medium">Exposure Settings</p>
                    <div className="flex flex-wrap gap-2 text-[11px] text-neutral-200 font-mono mt-1">
                      {photo.metadata.focalLength && (
                        <span className="bg-white/10 px-1.5 py-0.5 rounded">
                          {photo.metadata.focalLength}
                        </span>
                      )}
                      {photo.metadata.aperture && (
                        <span className="bg-white/10 px-1.5 py-0.5 rounded">
                          {photo.metadata.aperture}
                        </span>
                      )}
                      {photo.metadata.shutterSpeed && (
                        <span className="bg-white/10 px-1.5 py-0.5 rounded">
                          {photo.metadata.shutterSpeed}
                        </span>
                      )}
                      {photo.metadata.iso && (
                        <span className="bg-white/10 px-1.5 py-0.5 rounded">
                          ISO {photo.metadata.iso}
                        </span>
                      )}
                    </div>
                  </div>
                </div>
              )}

              {/* Tags */}
              {photo.tags && photo.tags.length > 0 && (
                <div className="flex items-start gap-3 pt-2 border-t border-white/10">
                  <Tag className="w-4 h-4 text-neutral-400 shrink-0 mt-0.5" />
                  <div>
                    <p className="text-neutral-400 font-medium mb-1.5">Tags</p>
                    <div className="flex flex-wrap gap-1.5">
                      {photo.tags.map((tag) => (
                        <span
                          key={tag}
                          className="px-2 py-0.5 rounded text-[11px] bg-white/10 text-neutral-300"
                        >
                          #{tag}
                        </span>
                      ))}
                    </div>
                  </div>
                </div>
              )}
            </div>
          </aside>
        )}
      </div>

      {/* Mobile Bottom Info Drawer / Strip: shown when showInfo is active */}
      {showInfo && (
        <div className="md:hidden flex flex-col p-4 bg-neutral-900/95 border-t border-white/10 text-xs text-neutral-300 space-y-2 z-20 max-h-48 overflow-y-auto">
          <div className="flex items-center justify-between">
            <span className="font-semibold text-white truncate max-w-[220px]">
              {photo.title}
            </span>
            <span className="font-mono text-[11px] text-neutral-400">{formatBytes(photo.size)}</span>
          </div>
          <div className="flex items-center gap-3 text-[11px] text-neutral-400">
            <span>
              {photo.takenAt ? formatDate(photo.takenAt) : photo.fileModifiedAt ? 'Modified: ' + formatDate(photo.fileModifiedAt) : 'Undated'}
            </span>
            <span>•</span>
            <span>{photo.folder ? photo.folder : 'Root (/media)'}</span>
            {photo.metadata.width > 0 && (
              <>
                <span>•</span>
                <span>
                  {photo.metadata.width}×{photo.metadata.height}
                </span>
              </>
            )}
          </div>
          {photo.metadata.camera && (
            <div className="text-[11px] text-neutral-400 flex items-center gap-1.5">
              <Camera className="w-3.5 h-3.5 shrink-0" />
              <span>{photo.metadata.camera}</span>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
