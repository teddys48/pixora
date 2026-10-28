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
  takenAt?: string // EXIF capture date if available
  size: number // file size in bytes
  folderId?: string
  tags?: string[]
  metadata: PhotoMetadata
}

export interface PhotoResponse {
  id: string
  relativePath: string
  originalFilename: string
  mediaType: string
  fileSize: number
  width?: number
  height?: number
  takenAt?: string
  fileModifiedAt: string
  createdAt: string
  updatedAt: string
  url: string
  thumbnailUrl: string
}

export interface PhotoListResponse {
  photos: PhotoResponse[]
  total: number
  limit: number
  offset: number
}

export interface ScanStatus {
  status: 'idle' | 'running' | 'completed' | 'failed'
  scanned: number
  indexed: number
  updated: number
  skipped: number
  failed: number
  errors?: string[]
  startedAt?: string
  finishedAt?: string
  durationMs?: number
}

export interface UploadFileError {
  filename: string
  error: string
}

export interface UploadResponse {
  photos: PhotoResponse[]
  errors?: UploadFileError[]
  message?: string
}

export type PhotoSortOption = 'date-desc' | 'date-asc' | 'title-asc' | 'title-desc' | 'size-desc'

export type PhotoViewMode = 'grid' | 'list'
