export interface PhotoMetadata {
  camera?: string
  lens?: string
  focalLength?: string
  aperture?: string
  shutterSpeed?: string
  iso?: number
  width: number
  height: number
}

export interface Photo {
  id: string
  title: string
  filename: string
  url: string
  thumbnailUrl: string
  fallbackDataUrl?: string
  date: string // ISO string: e.g. "2026-10-18T14:32:00Z"
  size: number // file size in bytes
  folderId?: string
  tags?: string[]
  metadata: PhotoMetadata
}

export type PhotoSortOption = 'date-desc' | 'date-asc' | 'title-asc' | 'title-desc' | 'size-desc'

export type PhotoViewMode = 'grid' | 'list'
