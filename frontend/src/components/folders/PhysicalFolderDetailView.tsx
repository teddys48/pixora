import { useState, useEffect, useCallback, useMemo } from 'react'
import { FolderContents, PhysicalFolder } from '@/types/folder'
import { Photo, PhotoViewMode } from '@/types/photo'
import { PhysicalFolderCard } from '@/components/folders/PhysicalFolderCard'
import { PhotoCard } from '@/components/gallery/PhotoCard'
import { Button } from '@/components/ui/Button'
import { useInfiniteScroll } from '@/hooks/useInfiniteScroll'
import { InfiniteScrollFooter } from '@/components/gallery/InfiniteScrollFooter'
import { Input } from '@/components/ui/Input'
import { fetchFolderContents, fetchFolderPhotos, triggerScan } from '@/lib/api'
import {
  ArrowLeft,
  Folder as FolderIcon,
  FolderTree,
  ChevronRight,
  Images,
  Search,
  Grid,
  List,
  RefreshCw,
  AlertCircle,
  CheckCircle2,
  HardDrive,
} from 'lucide-react'

interface PhysicalFolderDetailViewProps {
  path: string
  onNavigate: (newPath: string) => void
  onSelectPhoto: (photo: Photo, photosList: Photo[]) => void
}

export function PhysicalFolderDetailView({
  path,
  onNavigate,
  onSelectPhoto,
}: PhysicalFolderDetailViewProps) {
  const [contents, setContents] = useState<FolderContents | null>(null)
  const [photos, setPhotos] = useState<Photo[]>([])
  const [totalPhotos, setTotalPhotos] = useState(0)
  const [hasMore, setHasMore] = useState(false)
  const [nextCursor, setNextCursor] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loadMoreError, setLoadMoreError] = useState<string | null>(null)

  const [searchQuery, setSearchQuery] = useState('')
  const [viewMode, setViewMode] = useState<PhotoViewMode>('grid')

  const [isScanning, setIsScanning] = useState(false)
  const [scanMessage, setScanMessage] = useState<string | null>(null)

  const loadFolder = useCallback(async () => {
    setIsLoading(true)
    setError(null)
    setLoadMoreError(null)
    setScanMessage(null)
    try {
      const data = await fetchFolderContents(path, { limit: 50 })
      setContents(data)
      setPhotos(Array.isArray(data.photos) ? data.photos : [])
      setTotalPhotos(data.totalPhotos || 0)
      setHasMore(Boolean(data.hasMore))
      setNextCursor(data.nextCursor ?? null)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load folder contents'
      setError(msg)
    } finally {
      setIsLoading(false)
    }
  }, [path])

  useEffect(() => {
    loadFolder()
  }, [loadFolder])

  const loadMorePhotos = useCallback(async () => {
    if (isLoadingMore || !hasMore || !nextCursor) return
    setIsLoadingMore(true)
    setLoadMoreError(null)
    try {
      const res = await fetchFolderPhotos(path, { limit: 50, cursor: nextCursor })
      setPhotos((prev) => {
        const existing = new Set(prev.map((p) => p.id))
        const unique = (res.photos || []).filter((p) => !existing.has(p.id))
        return [...prev, ...unique]
      })
      setTotalPhotos(res.total || 0)
      setHasMore(Boolean(res.hasMore))
      setNextCursor(res.nextCursor ?? null)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load more photos'
      setLoadMoreError(msg)
    } finally {
      setIsLoadingMore(false)
    }
  }, [path, isLoadingMore, hasMore, nextCursor])

  const { sentinelRef } = useInfiniteScroll({
    onLoadMore: loadMorePhotos,
    hasMore,
    isLoading: isLoadingMore,
    disabled: Boolean(searchQuery.trim()),
  })

  const handleScanFolder = async () => {
    setIsScanning(true)
    setError(null)
    try {
      const status = await triggerScan(true, path)
      if (status.status === 'completed') {
        const indexed = status.indexed || 0
        setScanMessage(
          indexed > 0
            ? `Media scan completed: Indexed ${indexed} new photo${indexed === 1 ? '' : 's'}.`
            : 'Media scan completed: All media files in this folder are up to date.'
        )
      } else {
        setScanMessage('Media scan completed.')
      }
      // Reload folder data after scan
      await loadFolder()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to run media scan'
      setError(msg)
    } finally {
      setIsScanning(false)
    }
  }

  const filteredPhotos = useMemo(() => {
    if (!searchQuery.trim()) return photos
    const q = searchQuery.toLowerCase().trim()
    return photos.filter(
      (p) =>
        p.title.toLowerCase().includes(q) ||
        p.filename.toLowerCase().includes(q)
    )
  }, [photos, searchQuery])

  if (isLoading) {
    return (
      <div className="space-y-6 animate-pulse">
        <div className="h-6 w-48 bg-muted/60 rounded" />
        <div className="h-10 w-72 bg-muted/60 rounded" />
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-48 rounded-xl bg-muted/40 border border-border/60" />
          ))}
        </div>
      </div>
    )
  }

  if (error || !contents) {
    return (
      <div className="flex flex-col items-center justify-center p-14 text-center border border-dashed border-border rounded-2xl bg-card/40 my-8">
        <AlertCircle className="w-10 h-10 text-destructive mb-3" />
        <h3 className="text-lg font-semibold text-foreground mb-1">Failed to load folder</h3>
        <p className="text-sm text-muted-foreground max-w-sm mb-5 leading-relaxed">
          {error || 'The requested physical folder could not be found or accessed.'}
        </p>
        <div className="flex items-center gap-3">
          <Button onClick={loadFolder} variant="outline" size="sm">
            Try Again
          </Button>
          <Button onClick={() => onNavigate('')} size="sm">
            &larr; Back to all folders
          </Button>
        </div>
      </div>
    )
  }

  const breadcrumbs = Array.isArray(contents.breadcrumbs) ? contents.breadcrumbs : []
  const subfolders = Array.isArray(contents.folders) ? contents.folders : []

  return (
    <div className="space-y-6">
      {/* Breadcrumb Navigation Bar */}
      <nav aria-label="Folder Breadcrumbs" className="flex items-center flex-wrap gap-1.5 text-xs text-muted-foreground">
        {breadcrumbs.map((crumb, idx) => {
          const isLast = idx === breadcrumbs.length - 1
          return (
            <div key={crumb.path || 'root'} className="flex items-center gap-1.5">
              {idx > 0 && <ChevronRight className="w-3.5 h-3.5 opacity-50 shrink-0" />}
              {isLast ? (
                <span className="font-semibold text-foreground truncate max-w-[200px]" title={crumb.name}>
                  {crumb.name}
                </span>
              ) : (
                <button
                  type="button"
                  onClick={() => onNavigate(crumb.path)}
                  className="hover:text-foreground hover:underline transition-colors truncate max-w-[150px]"
                  title={`Navigate to ${crumb.name}`}
                >
                  {crumb.name}
                </button>
              )}
            </div>
          )
        })}
      </nav>

      {/* Folder Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4 border-b border-border/60">
        <div>
          <div className="flex items-center gap-2.5">
            {contents.parentPath !== null && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => onNavigate(contents.parentPath || '')}
                className="gap-1.5 h-8 px-2 text-xs text-muted-foreground hover:text-foreground"
                title="Go to parent directory"
              >
                <ArrowLeft className="w-3.5 h-3.5" />
                <span>Up</span>
              </Button>
            )}

            <div className="flex items-center gap-2 flex-wrap">
              <h1 className="text-2xl font-bold tracking-tight text-foreground break-all">
                {contents.name}
              </h1>
              <span className="flex items-center gap-1 text-[11px] font-medium text-muted-foreground bg-muted px-2 py-0.5 rounded-full border border-border/60">
                <HardDrive className="w-3 h-3 text-primary opacity-80" />
                Physical Folder
              </span>
              <span className="text-[11px] font-normal text-muted-foreground bg-muted px-2 py-0.5 rounded-full border border-border/60">
                {totalPhotos} {totalPhotos === 1 ? 'photo' : 'photos'}
              </span>
            </div>
          </div>

          <p className="text-xs text-muted-foreground mt-1.5 font-mono">
            {contents.path ? `/media/${contents.path}` : '/media'}
          </p>
        </div>

        {/* Action Controls */}
        <div className="flex items-center gap-2 self-start sm:self-auto">
          <Button
            onClick={handleScanFolder}
            disabled={isScanning}
            variant="outline"
            size="sm"
            className="gap-1.5 h-9 text-xs"
            title="Scan this folder for new or modified media files"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${isScanning ? 'animate-spin' : ''}`} />
            <span>{isScanning ? 'Scanning...' : 'Scan Folder'}</span>
          </Button>
        </div>
      </div>

      {/* Success Banner */}
      {scanMessage && (
        <div className="flex items-center justify-between p-3 rounded-xl border border-emerald-500/20 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 text-xs animate-in fade-in">
          <div className="flex items-center gap-2">
            <CheckCircle2 className="w-4 h-4 shrink-0" />
            <span>{scanMessage}</span>
          </div>
          <button
            type="button"
            onClick={() => setScanMessage(null)}
            className="text-xs opacity-70 hover:opacity-100 transition-opacity ml-3"
          >
            &times;
          </button>
        </div>
      )}

      {/* Unindexed Files Alert Banner */}
      {contents.hasUnindexedFiles && (
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 p-3.5 rounded-xl border border-amber-500/30 bg-amber-500/10 text-amber-900 dark:text-amber-200 text-xs">
          <div className="flex items-center gap-2.5">
            <AlertCircle className="w-4 h-4 text-amber-600 dark:text-amber-400 shrink-0" />
            <div>
              <span className="font-semibold">Unindexed media discovered: </span>
              <span>
                This folder contains {contents.unindexedCount} media {contents.unindexedCount === 1 ? 'file' : 'files'} on disk that {contents.unindexedCount === 1 ? 'has' : 'have'} not been indexed yet.
              </span>
            </div>
          </div>
          <Button
            onClick={handleScanFolder}
            disabled={isScanning}
            size="sm"
            className="h-7 text-xs bg-amber-600 hover:bg-amber-700 text-white shrink-0 self-start sm:self-auto"
          >
            {isScanning ? 'Scanning...' : 'Scan Now'}
          </Button>
        </div>
      )}

      {/* Subfolders Section */}
      {subfolders.length > 0 && (
        <section aria-labelledby="subfolders-heading" className="space-y-3">
          <div className="flex items-center gap-2">
            <FolderTree className="w-4 h-4 text-primary" />
            <h2 id="subfolders-heading" className="text-sm font-semibold tracking-tight text-foreground">
              Subfolders ({subfolders.length})
            </h2>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
            {subfolders.map((subfolder: PhysicalFolder) => (
              <PhysicalFolderCard
                key={subfolder.path}
                folder={subfolder}
                onClick={() => onNavigate(subfolder.path)}
              />
            ))}
          </div>
        </section>
      )}

      {/* Photos Section */}
      <section aria-labelledby="photos-heading" className="space-y-4">
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <Images className="w-4 h-4 text-primary" />
            <h2 id="photos-heading" className="text-sm font-semibold tracking-tight text-foreground">
              Photos in this folder ({totalPhotos})
            </h2>
          </div>

          {photos.length > 0 && (
            <div className="flex items-center gap-2">
              {/* Search input within folder */}
              <div className="relative flex-1 sm:w-56">
                <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground" />
                <Input
                  type="text"
                  placeholder="Filter photos..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-8 h-8 text-xs bg-muted/30"
                />
              </div>

              {/* View mode toggle */}
              <div className="flex items-center border border-border/70 rounded-md p-0.5 bg-card">
                <button
                  type="button"
                  onClick={() => setViewMode('grid')}
                  aria-label="Grid view"
                  className={`p-1.5 rounded text-xs transition-colors ${
                    viewMode === 'grid'
                      ? 'bg-primary text-primary-foreground shadow-xs'
                      : 'text-muted-foreground hover:text-foreground'
                  }`}
                >
                  <Grid className="w-3.5 h-3.5" />
                </button>
                <button
                  type="button"
                  onClick={() => setViewMode('list')}
                  aria-label="List view"
                  className={`p-1.5 rounded text-xs transition-colors ${
                    viewMode === 'list'
                      ? 'bg-primary text-primary-foreground shadow-xs'
                      : 'text-muted-foreground hover:text-foreground'
                  }`}
                >
                  <List className="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          )}
        </div>

        {/* Photos Grid / List */}
        {photos.length > 0 ? (
          <div className="space-y-6">
            {filteredPhotos.length === 0 ? (
              <div className="p-8 text-center border border-dashed border-border rounded-xl bg-card/20 text-xs text-muted-foreground">
                No photos matching &quot;{searchQuery}&quot; found in this folder.
              </div>
            ) : (
              <div
                className={
                  viewMode === 'grid'
                    ? 'grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-3'
                    : 'flex flex-col gap-2'
                }
              >
                {filteredPhotos.map((photo) => (
                  <PhotoCard
                    key={photo.id}
                    photo={photo}
                    viewMode={viewMode}
                    onSelect={(p) => onSelectPhoto(p, filteredPhotos)}
                  />
                ))}
              </div>
            )}

            {/* Infinite Scroll Footer */}
            {!searchQuery.trim() && (
              <InfiniteScrollFooter
                sentinelRef={sentinelRef}
                hasMore={hasMore}
                isLoadingMore={isLoadingMore}
                error={loadMoreError}
                onRetry={loadMorePhotos}
                loadedCount={photos.length}
                totalCount={totalPhotos}
              />
            )}
          </div>
        ) : (
          /* Empty Photos State */
          <div className="flex flex-col items-center justify-center p-10 text-center border border-dashed border-border/80 rounded-2xl bg-card/20">
            <div className="flex items-center justify-center w-12 h-12 rounded-full bg-muted/60 text-muted-foreground mb-3">
              <FolderIcon className="w-6 h-6 stroke-[1.5]" />
            </div>
            <h3 className="text-sm font-semibold text-foreground mb-1">
              No photos directly in this folder
            </h3>
            <p className="text-xs text-muted-foreground max-w-sm mb-4 leading-relaxed">
              {subfolders.length > 0
                ? 'This directory has subfolders above, but no media files placed directly at this level.'
                : contents.hasUnindexedFiles
                ? 'Media files exist on disk in this directory. Run a scan to index them into Pixora.'
                : 'This folder is empty.'}
            </p>
            {contents.hasUnindexedFiles && (
              <Button
                onClick={handleScanFolder}
                disabled={isScanning}
                size="sm"
                className="gap-2 text-xs h-8 font-medium"
              >
                <RefreshCw className={`w-3.5 h-3.5 ${isScanning ? 'animate-spin' : ''}`} />
                <span>{isScanning ? 'Scanning...' : 'Scan Folder Media'}</span>
              </Button>
            )}
          </div>
        )}
      </section>
    </div>
  )
}
