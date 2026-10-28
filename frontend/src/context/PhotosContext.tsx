import React, { createContext, useContext, useState, useEffect, useCallback } from 'react'
import { Photo, PhotoSortOption, PhotoViewMode, ScanStatus } from '@/types/photo'
import { fetchPhotos, triggerScan, fetchScanStatus } from '@/lib/api'

interface PhotosContextType {
  photos: Photo[]
  totalPhotos: number
  isLoading: boolean
  isLoadingMore: boolean
  error: string | null
  sortOption: PhotoSortOption
  setSortOption: (option: PhotoSortOption) => void
  viewMode: PhotoViewMode
  setViewMode: (mode: PhotoViewMode) => void
  searchQuery: string
  setSearchQuery: (query: string) => void
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
  const [isLoading, setIsLoading] = useState(true)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [searchQuery, setSearchQuery] = useState('')
  const [sortOption, setSortOption] = useState<PhotoSortOption>('date-desc')
  const [viewMode, setViewMode] = useState<PhotoViewMode>('grid')

  const [isScanning, setIsScanning] = useState(false)
  const [scanResult, setScanResult] = useState<ScanStatus | null>(null)

  const loadPhotos = useCallback(async (isInitial = false) => {
    if (isInitial) {
      setIsLoading(true)
      setError(null)
    }

    try {
      const [sortBy, order] = sortOption.startsWith('title')
        ? ['filename', sortOption.endsWith('asc') ? 'asc' : 'desc']
        : sortOption.startsWith('size')
        ? ['size', 'desc']
        : ['date', sortOption.endsWith('asc') ? 'asc' : 'desc']

      const result = await fetchPhotos({
        limit: 50,
        offset: 0,
        sort: sortBy,
        order: order,
      })

      setPhotos(result.photos)
      setTotalPhotos(result.total)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load photos'
      setError(msg)
    } finally {
      if (isInitial) {
        setIsLoading(false)
      }
    }
  }, [sortOption])

  useEffect(() => {
    loadPhotos(true)
  }, [loadPhotos])

  const loadMore = useCallback(async () => {
    if (isLoadingMore || photos.length >= totalPhotos) return
    setIsLoadingMore(true)

    try {
      const [sortBy, order] = sortOption.startsWith('title')
        ? ['filename', sortOption.endsWith('asc') ? 'asc' : 'desc']
        : sortOption.startsWith('size')
        ? ['size', 'desc']
        : ['date', sortOption.endsWith('asc') ? 'asc' : 'desc']

      const result = await fetchPhotos({
        limit: 50,
        offset: photos.length,
        sort: sortBy,
        order: order,
      })

      setPhotos((prev) => [...prev, ...result.photos])
      setTotalPhotos(result.total)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load more photos'
      setError(msg)
    } finally {
      setIsLoadingMore(false)
    }
  }, [isLoadingMore, photos.length, totalPhotos, sortOption])

  const refreshPhotos = useCallback(async () => {
    await loadPhotos(false)
  }, [loadPhotos])

  const handleScan = useCallback(async () => {
    if (isScanning) return
    setIsScanning(true)
    setScanResult(null)

    try {
      const initial = await triggerScan(false)
      if (initial.status === 'completed' || initial.status === 'failed') {
        setScanResult(initial)
        setIsScanning(false)
        await loadPhotos(false)
        return
      }

      const pollInterval = window.setInterval(async () => {
        try {
          const current = await fetchScanStatus()
          if (current.status === 'completed' || current.status === 'failed') {
            window.clearInterval(pollInterval)
            setScanResult(current)
            setIsScanning(false)
            await loadPhotos(false)
          }
        } catch {
          window.clearInterval(pollInterval)
          setIsScanning(false)
        }
      }, 1200)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to initiate scan'
      setError(msg)
      setIsScanning(false)
    }
  }, [isScanning, loadPhotos])

  return (
    <PhotosContext.Provider
      value={{
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
