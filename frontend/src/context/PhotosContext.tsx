import React, { createContext, useContext, useState, useEffect, useCallback, useRef, useMemo } from 'react'
import { Photo, PhotoSortOption, PhotoViewMode, ScanStatus } from '@/types/photo'
import { fetchPhotos, triggerScan, fetchScanStatus } from '@/lib/api'

interface PhotosContextType {
  photos: Photo[]
  totalPhotos: number
  nextCursor: string | null
  hasMore: boolean
  isLoading: boolean
  isLoadingMore: boolean
  error: string | null
  loadMoreError: string | null
  sortOption: PhotoSortOption
  setSortOption: (option: PhotoSortOption) => void
  viewMode: PhotoViewMode
  setViewMode: (mode: PhotoViewMode) => void
  searchQuery: string
  setSearchQuery: (query: string) => void
  startDate: string
  setStartDate: (date: string) => void
  endDate: string
  setEndDate: (date: string) => void
  folderFilter: string
  setFolderFilter: (folder: string) => void
  albumFilter: string
  setAlbumFilter: (albumId: string) => void
  mediaTypeFilter: string
  setMediaTypeFilter: (mediaType: string) => void
  undatedOnly: boolean
  setUndatedOnly: (undated: boolean) => void
  hasActiveFilters: boolean
  activeFiltersCount: number
  clearFilters: () => void
  resetAll: () => void
  isScanning: boolean
  scanResult: ScanStatus | null
  setScanResult: (res: ScanStatus | null) => void
  loadPhotos: (isInitial?: boolean) => Promise<void>
  loadMore: () => Promise<void>
  refreshPhotos: () => Promise<void>
  handleScan: () => Promise<void>
}

const PhotosContext = createContext<PhotosContextType | undefined>(undefined)

export function PhotosProvider({ children }: { children: React.ReactNode }) {
  const [photos, setPhotos] = useState<Photo[]>([])
  const [totalPhotos, setTotalPhotos] = useState(0)
  const [nextCursor, setNextCursor] = useState<string | null>(null)
  const [hasMore, setHasMore] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loadMoreError, setLoadMoreError] = useState<string | null>(null)

  // Search and discovery filter states
  const [searchQuery, setSearchQuery] = useState('')
  const [debouncedSearchQuery, setDebouncedSearchQuery] = useState('')
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const [folderFilter, setFolderFilter] = useState('')
  const [albumFilter, setAlbumFilter] = useState('')
  const [mediaTypeFilter, setMediaTypeFilter] = useState('')
  const [undatedOnly, setUndatedOnly] = useState(false)

  const [sortOption, setSortOption] = useState<PhotoSortOption>('date-desc')
  const [viewMode, setViewMode] = useState<PhotoViewMode>('grid')

  const [isScanning, setIsScanning] = useState(false)
  const [scanResult, setScanResult] = useState<ScanStatus | null>(null)

  // In-flight request cancellation
  const abortControllerRef = useRef<AbortController | null>(null)

  // Debounce search query changes by 300ms
  useEffect(() => {
    const handler = setTimeout(() => {
      setDebouncedSearchQuery(searchQuery)
    }, 300)
    return () => clearTimeout(handler)
  }, [searchQuery])

  // Count active filters (excluding default sort/view)
  const activeFiltersCount = useMemo(() => {
    let count = 0
    if (debouncedSearchQuery.trim()) count++
    if (startDate) count++
    if (endDate) count++
    if (folderFilter) count++
    if (albumFilter) count++
    if (mediaTypeFilter) count++
    if (undatedOnly) count++
    return count
  }, [debouncedSearchQuery, startDate, endDate, folderFilter, albumFilter, mediaTypeFilter, undatedOnly])

  const hasActiveFilters = activeFiltersCount > 0

  const clearFilters = useCallback(() => {
    setStartDate('')
    setEndDate('')
    setFolderFilter('')
    setAlbumFilter('')
    setMediaTypeFilter('')
    setUndatedOnly(false)
  }, [])

  const resetAll = useCallback(() => {
    setSearchQuery('')
    setDebouncedSearchQuery('')
    clearFilters()
  }, [clearFilters])

  const mapSortOption = useCallback((option: PhotoSortOption): [string, string] => {
    switch (option) {
      case 'date-desc':
        return ['date', 'desc']
      case 'date-asc':
        return ['date', 'asc']
      case 'title-asc':
        return ['filename', 'asc']
      case 'title-desc':
        return ['filename', 'desc']
      case 'size-desc':
        return ['size', 'desc']
      case 'size-asc':
        return ['size', 'asc']
      case 'modified-desc':
        return ['modified', 'desc']
      case 'modified-asc':
        return ['modified', 'asc']
      default:
        return ['date', 'desc']
    }
  }, [])

  const loadPhotos = useCallback(
    async (isInitial = false) => {
      // Abort previous in-flight request
      if (abortControllerRef.current) {
        abortControllerRef.current.abort()
      }
      const controller = new AbortController()
      abortControllerRef.current = controller

      if (isInitial) {
        setIsLoading(true)
        setError(null)
      }
      setLoadMoreError(null)

      try {
        const [sortBy, order] = mapSortOption(sortOption)

        const result = await fetchPhotos(
          {
            limit: 50,
            sort: sortBy,
            order: order,
            q: debouncedSearchQuery.trim() || undefined,
            startDate: startDate || undefined,
            endDate: endDate || undefined,
            folder: folderFilter || undefined,
            albumId: albumFilter || undefined,
            mediaType: mediaTypeFilter || undefined,
            isUndated: undatedOnly ? true : undefined,
          },
          controller.signal
        )

        setPhotos(result.photos)
        setTotalPhotos(result.total)
        setNextCursor(result.nextCursor ?? null)
        setHasMore(Boolean(result.hasMore))
      } catch (err: unknown) {
        if (err instanceof DOMException && err.name === 'AbortError') {
          return // Request was intentionally cancelled
        }
        const msg = err instanceof Error ? err.message : 'Failed to load photos'
        setError(msg)
      } finally {
        if (isInitial) {
          setIsLoading(false)
        }
      }
    },
    [
      sortOption,
      debouncedSearchQuery,
      startDate,
      endDate,
      folderFilter,
      albumFilter,
      mediaTypeFilter,
      undatedOnly,
      mapSortOption,
    ]
  )

  useEffect(() => {
    loadPhotos(true)
  }, [loadPhotos])

  const loadMore = useCallback(async () => {
    if (isLoadingMore || !hasMore || !nextCursor) return
    setIsLoadingMore(true)
    setLoadMoreError(null)

    try {
      const [sortBy, order] = mapSortOption(sortOption)

      const result = await fetchPhotos({
        limit: 50,
        cursor: nextCursor,
        sort: sortBy,
        order: order,
        q: debouncedSearchQuery.trim() || undefined,
        startDate: startDate || undefined,
        endDate: endDate || undefined,
        folder: folderFilter || undefined,
        albumId: albumFilter || undefined,
        mediaType: mediaTypeFilter || undefined,
        isUndated: undatedOnly ? true : undefined,
      })

      setPhotos((prev) => {
        const existing = new Set(prev.map((p) => p.id))
        const unique = result.photos.filter((p) => !existing.has(p.id))
        return [...prev, ...unique]
      })
      setTotalPhotos(result.total)
      setNextCursor(result.nextCursor ?? null)
      setHasMore(Boolean(result.hasMore))
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load more photos'
      setLoadMoreError(msg)
    } finally {
      setIsLoadingMore(false)
    }
  }, [
    isLoadingMore,
    hasMore,
    nextCursor,
    sortOption,
    debouncedSearchQuery,
    startDate,
    endDate,
    folderFilter,
    albumFilter,
    mediaTypeFilter,
    undatedOnly,
    mapSortOption,
  ])

  const refreshPhotos = useCallback(async () => {
    await loadPhotos(false)
  }, [loadPhotos])

  const handleScan = useCallback(async () => {
    setIsScanning(true)
    try {
      const initial = await triggerScan(false)
      setScanResult(initial)

      const checkInterval = setInterval(async () => {
        try {
          const status = await fetchScanStatus()
          setScanResult(status)
          if (status.status !== 'running') {
            clearInterval(checkInterval)
            setIsScanning(false)
            await refreshPhotos()
          }
        } catch {
          clearInterval(checkInterval)
          setIsScanning(false)
        }
      }, 1000)
    } catch (err: unknown) {
      setIsScanning(false)
      const msg = err instanceof Error ? err.message : 'Failed to start media library scan'
      setError(msg)
    }
  }, [refreshPhotos])

  return (
    <PhotosContext.Provider
      value={{
        photos,
        totalPhotos,
        nextCursor,
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
        refreshPhotos,
        handleScan,
      }}
    >
      {children}
    </PhotosContext.Provider>
  )
}

export function usePhotos() {
  const context = useContext(PhotosContext)
  if (!context) {
    throw new Error('usePhotos must be used within a PhotosProvider')
  }
  return context
}
