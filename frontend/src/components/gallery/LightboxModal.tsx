import { useEffect, useState, useCallback } from 'react'
import { Photo } from '@/types/photo'
import { FallbackImage } from '@/components/ui/FallbackImage'
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
} from 'lucide-react'
import { cn } from '@/lib/utils'

interface LightboxModalProps {
  photo: Photo | null
  photos: Photo[]
  isOpen: boolean
  onClose: () => void
  onNavigate: (photo: Photo) => void
}

export function LightboxModal({
  photo,
  photos,
  isOpen,
  onClose,
  onNavigate,
}: LightboxModalProps) {
  const [showInfo, setShowInfo] = useState(true)

  const currentIndex = photo ? photos.findIndex((p) => p.id === photo.id) : -1
  const hasPrev = currentIndex > 0
  const hasNext = currentIndex >= 0 && currentIndex < photos.length - 1

  const handlePrev = useCallback(() => {
    if (hasPrev) {
      onNavigate(photos[currentIndex - 1])
    }
  }, [hasPrev, photos, currentIndex, onNavigate])

  const handleNext = useCallback(() => {
    if (hasNext) {
      onNavigate(photos[currentIndex + 1])
    }
  }, [hasNext, photos, currentIndex, onNavigate])

  useEffect(() => {
    if (!isOpen) return

    const originalOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      } else if (e.key === 'ArrowLeft') {
        handlePrev()
      } else if (e.key === 'ArrowRight') {
        handleNext()
      } else if (e.key.toLowerCase() === 'i') {
        setShowInfo((prev) => !prev)
      }
    }

    window.addEventListener('keydown', handleKeyDown)

    return () => {
      document.body.style.overflow = originalOverflow
      window.removeEventListener('keydown', handleKeyDown)
    }
  }, [isOpen, onClose, handlePrev, handleNext])

  if (!isOpen || !photo) return null

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={`Photo viewer: ${photo.title}`}
      className="fixed inset-0 z-50 flex flex-col bg-black/95 text-white backdrop-blur-md select-none"
    >
      {/* Top Header Bar */}
      <div className="flex h-14 items-center justify-between px-4 border-b border-white/10 z-10 shrink-0 bg-black/40">
        <div className="flex items-center gap-3 min-w-0">
          <span className="text-xs text-neutral-400 font-mono">
            {currentIndex + 1} / {photos.length}
          </span>
          <span className="text-sm font-medium text-neutral-200 truncate max-w-xs md:max-w-md">
            {photo.title}
          </span>
        </div>

        <div className="flex items-center gap-2">
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
          <button
            onClick={onClose}
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
            className="absolute left-4 top-1/2 -translate-y-1/2 z-20 flex h-11 w-11 items-center justify-center rounded-full bg-black/60 text-white/80 hover:text-white hover:bg-black/90 backdrop-blur-sm border border-white/10 transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-white"
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
            className="absolute right-4 top-1/2 -translate-y-1/2 z-20 flex h-11 w-11 items-center justify-center rounded-full bg-black/60 text-white/80 hover:text-white hover:bg-black/90 backdrop-blur-sm border border-white/10 transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-white"
            title="Next (Arrow Right)"
          >
            <ChevronRight className="h-6 w-6" />
          </button>
        )}

        {/* Center Image Display */}
        <div className="flex-1 flex items-center justify-center p-4 md:p-8 overflow-hidden">
          <div className="relative max-h-full max-w-full flex items-center justify-center">
            <FallbackImage
              src={photo.url}
              alt={photo.title}
              className="max-h-[82vh] max-w-[88vw] w-auto h-auto rounded-lg shadow-2xl flex items-center justify-center"
              imgClassName="max-h-[82vh] max-w-[88vw] w-auto h-auto object-contain rounded-lg shadow-2xl"
            />
          </div>
        </div>

        {/* Metadata Sidebar (Toggleable) */}
        {showInfo && (
          <aside className="w-80 shrink-0 border-l border-white/10 bg-neutral-900/95 backdrop-blur-md p-6 overflow-y-auto space-y-6 hidden md:block animate-in slide-in-from-right-4 duration-200">
            <div>
              <h2 className="text-base font-semibold text-white tracking-tight">{photo.title}</h2>
              <p className="text-xs text-neutral-400 font-mono mt-1 break-all">{photo.filename}</p>
            </div>

            {/* Photo Details List */}
            <div className="space-y-4 text-xs">
              <div className="flex items-start gap-3">
                <Calendar className="w-4 h-4 text-neutral-400 shrink-0 mt-0.5" />
                <div>
                  <p className="text-neutral-400 font-medium">Capture Date</p>
                  <p className="text-neutral-200">{formatDate(photo.date)}</p>
                </div>
              </div>

              {photo.metadata.camera && (
                <div className="flex items-start gap-3">
                  <Camera className="w-4 h-4 text-neutral-400 shrink-0 mt-0.5" />
                  <div>
                    <p className="text-neutral-400 font-medium">Camera</p>
                    <p className="text-neutral-200">{photo.metadata.camera}</p>
                    {photo.metadata.lens && (
                      <p className="text-neutral-400 text-[11px] mt-0.5">{photo.metadata.lens}</p>
                    )}
                  </div>
                </div>
              )}

              {(photo.metadata.aperture || photo.metadata.shutterSpeed || photo.metadata.iso) && (
                <div className="flex items-start gap-3">
                  <Aperture className="w-4 h-4 text-neutral-400 shrink-0 mt-0.5" />
                  <div>
                    <p className="text-neutral-400 font-medium">Exposure Settings</p>
                    <div className="flex flex-wrap gap-2 text-[11px] text-neutral-200 font-mono mt-1">
                      {photo.metadata.focalLength && <span>{photo.metadata.focalLength}</span>}
                      {photo.metadata.aperture && <span>{photo.metadata.aperture}</span>}
                      {photo.metadata.shutterSpeed && <span>{photo.metadata.shutterSpeed}</span>}
                      {photo.metadata.iso && <span>ISO {photo.metadata.iso}</span>}
                    </div>
                  </div>
                </div>
              )}

              <div className="flex items-start gap-3">
                <Maximize className="w-4 h-4 text-neutral-400 shrink-0 mt-0.5" />
                <div>
                  <p className="text-neutral-400 font-medium">Resolution</p>
                  <p className="text-neutral-200 font-mono">
                    {photo.metadata.width} × {photo.metadata.height} px
                  </p>
                </div>
              </div>

              <div className="flex items-start gap-3">
                <HardDrive className="w-4 h-4 text-neutral-400 shrink-0 mt-0.5" />
                <div>
                  <p className="text-neutral-400 font-medium">File Size</p>
                  <p className="text-neutral-200 font-mono">{formatBytes(photo.size)}</p>
                </div>
              </div>

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

      {/* Mobile Bottom Info Strip */}
      <div className="md:hidden flex items-center justify-between px-4 py-2 bg-neutral-900/90 border-t border-white/10 text-xs text-neutral-300">
        <span className="truncate max-w-[200px]">{photo.title}</span>
        <span className="font-mono text-[11px] text-neutral-400">{formatBytes(photo.size)}</span>
      </div>
    </div>
  )
}
