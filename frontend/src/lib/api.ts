import { HealthResponse } from '@/types/health'
import {
  Photo,
  PhotoResponse,
  PhotoListResponse,
  TimelineResponse,
  ScanStatus,
  UploadResponse,
} from '@/types/photo'
import { Folder, PhysicalFolder, FolderContents } from '@/types/folder'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api'

export function toFrontendPhoto(p: PhotoResponse): Photo {
  const isUndated = p.isUndated ?? (p.takenAt ? false : true)
  const dateSource =
    p.dateSource ?? (p.takenAt ? 'exif' : p.fileModifiedAt ? 'file_modified' : 'none')
  const date = p.takenAt || p.fileModifiedAt || p.createdAt || ''
  // Clean filename without extension for title
  const title = p.originalFilename.replace(/\.[^/.]+$/, '')
  return {
    id: p.id,
    title: title,
    filename: p.originalFilename,
    url: p.url,
    thumbnailUrl: p.thumbnailUrl,
    date: date,
    dateSource: dateSource,
    takenAt: p.takenAt,
    fileModifiedAt: p.fileModifiedAt,
    isUndated: isUndated,
    size: p.fileSize,
    folder: p.folder,
    metadata: {
      width: p.width || 0,
      height: p.height || 0,
      camera: p.exif?.camera,
      lens: p.exif?.lens,
      focalLength: p.exif?.focalLength,
      aperture: p.exif?.aperture,
      shutterSpeed: p.exif?.shutterSpeed,
      iso: p.exif?.iso,
    },
    prevId: p.prevId,
    nextId: p.nextId,
  }
}

export async function fetchHealth(signal?: AbortSignal): Promise<HealthResponse> {
  const response = await fetch(`${API_BASE_URL}/health`, {
    method: 'GET',
    headers: {
      Accept: 'application/json',
    },
    signal,
  })

  if (!response.ok) {
    throw new Error(`Health check failed with HTTP ${response.status}`)
  }

  return response.json()
}

export interface FetchPhotosOptions {
  limit?: number
  offset?: number
  sort?: string
  order?: string
}

export async function fetchPhotos(
  options: FetchPhotosOptions = {},
  signal?: AbortSignal
): Promise<{ photos: Photo[]; total: number; limit: number; offset: number }> {
  const params = new URLSearchParams()
  if (options.limit !== undefined) params.set('limit', options.limit.toString())
  if (options.offset !== undefined) params.set('offset', options.offset.toString())
  if (options.sort) params.set('sort', options.sort)
  if (options.order) params.set('order', options.order)

  const url = `${API_BASE_URL}/photos${params.toString() ? `?${params.toString()}` : ''}`
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      Accept: 'application/json',
    },
    signal,
  })

  if (!response.ok) {
    const errorBody = await response.json().catch(() => null)
    throw new Error(errorBody?.message || `Failed to fetch photos with status ${response.status}`)
  }

  const data: PhotoListResponse = await response.json()
  return {
    photos: (data.photos || []).map(toFrontendPhoto),
    total: data.total || 0,
    limit: data.limit || 50,
    offset: data.offset || 0,
  }
}

export interface FetchTimelineOptions {
  limit?: number
  offset?: number
  cursor?: string
}

export interface TimelineData {
  photos: Photo[]
  groups: {
    date: string
    label: string
    year?: number
    month?: string
    day?: number
    isUndated: boolean
    photos: Photo[]
  }[]
  total: number
  limit: number
  offset: number
  nextCursor?: string
  hasMore: boolean
}

export async function fetchTimeline(
  options: FetchTimelineOptions = {},
  signal?: AbortSignal
): Promise<TimelineData> {
  const params = new URLSearchParams()
  if (options.limit !== undefined) params.set('limit', options.limit.toString())
  if (options.offset !== undefined) params.set('offset', options.offset.toString())
  if (options.cursor) params.set('cursor', options.cursor)

  const url = `${API_BASE_URL}/timeline${params.toString() ? `?${params.toString()}` : ''}`
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      Accept: 'application/json',
    },
    signal,
  })

  if (!response.ok) {
    const errorBody = await response.json().catch(() => null)
    throw new Error(errorBody?.message || `Failed to fetch timeline with status ${response.status}`)
  }

  const data: TimelineResponse = await response.json()
  return {
    photos: (data.photos || []).map(toFrontendPhoto),
    groups: (data.groups || []).map((g) => ({
      ...g,
      photos: (g.photos || []).map(toFrontendPhoto),
    })),
    total: data.total || 0,
    limit: data.limit || 50,
    offset: data.offset || 0,
    nextCursor: data.nextCursor,
    hasMore: Boolean(data.hasMore),
  }
}

export async function fetchPhoto(
  id: string,
  options?: { collectionType?: string; collectionId?: string },
  signal?: AbortSignal
): Promise<Photo> {
  const params = new URLSearchParams()
  if (options?.collectionType) params.set('collectionType', options.collectionType)
  if (options?.collectionId) params.set('collectionId', options.collectionId)

  const url = `${API_BASE_URL}/photos/${encodeURIComponent(id)}${
    params.toString() ? `?${params.toString()}` : ''
  }`
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      Accept: 'application/json',
    },
    signal,
  })

  if (!response.ok) {
    const errorBody = await response.json().catch(() => null)
    throw new Error(errorBody?.message || `Failed to fetch photo with status ${response.status}`)
  }

  const data: PhotoResponse = await response.json()
  return toFrontendPhoto(data)
}

export async function uploadPhotos(
  files: File[],
  onProgress?: (progress: number) => void
): Promise<UploadResponse> {
  const formData = new FormData()
  files.forEach((file) => {
    formData.append('photos', file)
  })

  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `${API_BASE_URL}/photos/upload`)
    xhr.setRequestHeader('Accept', 'application/json')

    if (xhr.upload && onProgress) {
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) {
          const percentComplete = (e.loaded / e.total) * 100
          onProgress(percentComplete)
        }
      }
    }

    xhr.onload = () => {
      let data: UploadResponse
      try {
        data = JSON.parse(xhr.responseText)
      } catch {
        return reject(new Error('Invalid server response format'))
      }

      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(data)
      } else if (xhr.status === 207) {
        // Multi-status partial success
        resolve(data)
      } else {
        const errorMsg =
          (data as unknown as { message?: string })?.message ||
          `Upload failed with HTTP ${xhr.status}`
        reject(new Error(errorMsg))
      }
    }

    xhr.onerror = () => {
      reject(new Error('Network error during photo upload'))
    }

    xhr.ontimeout = () => {
      reject(new Error('Upload request timed out'))
    }

    xhr.send(formData)
  })
}

export async function triggerScan(
  sync = false,
  path?: string,
  refresh = false,
  signal?: AbortSignal
): Promise<ScanStatus> {
  const params = new URLSearchParams()
  if (sync) params.set('sync', 'true')
  if (path) params.set('path', path)
  if (refresh) params.set('refresh', 'true')

  const url = `${API_BASE_URL}/photos/scan${params.toString() ? `?${params.toString()}` : ''}`
  const response = await fetch(url, {
    method: 'POST',
    headers: {
      Accept: 'application/json',
    },
    signal,
  })

  if (!response.ok) {
    const errorBody = await response.json().catch(() => null)
    throw new Error(errorBody?.message || `Failed to trigger scan: HTTP ${response.status}`)
  }

  return response.json()
}

export async function fetchScanStatus(signal?: AbortSignal): Promise<ScanStatus> {
  const response = await fetch(`${API_BASE_URL}/photos/scan`, {
    method: 'GET',
    headers: {
      Accept: 'application/json',
    },
    signal,
  })

  if (!response.ok) {
    throw new Error(`Failed to get scan status: HTTP ${response.status}`)
  }

  return response.json()
}

// ==========================================
// Virtual Albums API (backed by SQLite)
// ==========================================

export interface BackendAlbum {
  id: string
  name: string
  description: string
  photoCount: number
  coverPhotoId?: string
  coverThumbnailUrl?: string
  createdAt: string
  updatedAt: string
}

export function toFrontendFolder(a: BackendAlbum): Folder {
  return {
    id: a.id,
    name: a.name,
    description: a.description,
    photoCount: a.photoCount || 0,
    coverUrl:
      a.coverThumbnailUrl ||
      (a.coverPhotoId
        ? `${API_BASE_URL}/photos/${encodeURIComponent(a.coverPhotoId)}/thumbnail`
        : undefined),
    createdAt: a.createdAt,
    updatedAt: a.updatedAt,
  }
}

export interface FetchAlbumsOptions {
  search?: string
  sort?: string
  order?: string
}

export async function fetchAlbums(
  options: FetchAlbumsOptions = {},
  signal?: AbortSignal
): Promise<Folder[]> {
  const params = new URLSearchParams()
  if (options.search) params.set('search', options.search)
  if (options.sort) params.set('sort', options.sort)
  if (options.order) params.set('order', options.order)

  const url = `${API_BASE_URL}/albums${params.toString() ? `?${params.toString()}` : ''}`
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      Accept: 'application/json',
    },
    signal,
  })

  if (!response.ok) {
    const errorBody = await response.json().catch(() => null)
    throw new Error(errorBody?.message || `Failed to fetch albums: HTTP ${response.status}`)
  }

  const data: { albums: BackendAlbum[] } = await response.json()
  return (data.albums || []).map(toFrontendFolder)
}

export async function fetchAlbum(id: string, signal?: AbortSignal): Promise<Folder> {
  const response = await fetch(`${API_BASE_URL}/albums/${encodeURIComponent(id)}`, {
    method: 'GET',
    headers: {
      Accept: 'application/json',
    },
    signal,
  })

  if (!response.ok) {
    const errorBody = await response.json().catch(() => null)
    throw new Error(errorBody?.message || `Failed to fetch album: HTTP ${response.status}`)
  }

  const a: BackendAlbum = await response.json()
  return toFrontendFolder(a)
}

export async function createAlbum(name: string, description = ''): Promise<Folder> {
  const response = await fetch(`${API_BASE_URL}/albums`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    body: JSON.stringify({ name, description }),
  })

  if (!response.ok) {
    const errorBody = await response.json().catch(() => null)
    throw new Error(errorBody?.message || `Failed to create album: HTTP ${response.status}`)
  }

  const a: BackendAlbum = await response.json()
  return toFrontendFolder(a)
}

export async function updateAlbum(
  id: string,
  name?: string,
  description?: string
): Promise<Folder> {
  const payload: { name?: string; description?: string } = {}
  if (name !== undefined) payload.name = name
  if (description !== undefined) payload.description = description

  const response = await fetch(`${API_BASE_URL}/albums/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    body: JSON.stringify(payload),
  })

  if (!response.ok) {
    const errorBody = await response.json().catch(() => null)
    throw new Error(errorBody?.message || `Failed to update album: HTTP ${response.status}`)
  }

  const a: BackendAlbum = await response.json()
  return toFrontendFolder(a)
}

export async function deleteAlbum(id: string): Promise<void> {
  const response = await fetch(`${API_BASE_URL}/albums/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    headers: {
      Accept: 'application/json',
    },
  })

  if (!response.ok) {
    const errorBody = await response.json().catch(() => null)
    throw new Error(errorBody?.message || `Failed to delete album: HTTP ${response.status}`)
  }
}

export interface FetchAlbumPhotosOptions {
  limit?: number
  offset?: number
}

export async function fetchAlbumPhotos(
  albumId: string,
  options: FetchAlbumPhotosOptions = {},
  signal?: AbortSignal
): Promise<{ photos: Photo[]; total: number; limit: number; offset: number }> {
  const params = new URLSearchParams()
  if (options.limit !== undefined) params.set('limit', options.limit.toString())
  if (options.offset !== undefined) params.set('offset', options.offset.toString())

  const url = `${API_BASE_URL}/albums/${encodeURIComponent(albumId)}/photos${
    params.toString() ? `?${params.toString()}` : ''
  }`
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      Accept: 'application/json',
    },
    signal,
  })

  if (!response.ok) {
    const errorBody = await response.json().catch(() => null)
    throw new Error(errorBody?.message || `Failed to fetch album photos: HTTP ${response.status}`)
  }

  const data: PhotoListResponse = await response.json()
  return {
    photos: (data.photos || []).map(toFrontendPhoto),
    total: data.total || 0,
    limit: data.limit || 50,
    offset: data.offset || 0,
  }
}

export async function addPhotosToAlbum(
  albumId: string,
  photoIds: string[]
): Promise<{ added: number; message: string }> {
  const response = await fetch(`${API_BASE_URL}/albums/${encodeURIComponent(albumId)}/photos`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    body: JSON.stringify({ photoIds }),
  })

  if (!response.ok) {
    const errorBody = await response.json().catch(() => null)
    throw new Error(errorBody?.message || `Failed to add photos to album: HTTP ${response.status}`)
  }

  return response.json()
}

export async function removePhotoFromAlbum(albumId: string, photoId: string): Promise<void> {
  const response = await fetch(
    `${API_BASE_URL}/albums/${encodeURIComponent(albumId)}/photos/${encodeURIComponent(photoId)}`,
    {
      method: 'DELETE',
      headers: {
        Accept: 'application/json',
      },
    }
  )

  if (!response.ok) {
    const errorBody = await response.json().catch(() => null)
    throw new Error(
      errorBody?.message || `Failed to remove photo from album: HTTP ${response.status}`
    )
  }
}

// ==========================================
// Physical Folders API (Filesystem browsing under STORAGE_ROOT)
// ==========================================

export interface FetchPhysicalFoldersOptions {
  path?: string
  sort?: string
  order?: string
}

export async function fetchPhysicalFolders(
  options: FetchPhysicalFoldersOptions = {},
  signal?: AbortSignal
): Promise<{ path: string; folders: PhysicalFolder[]; total: number }> {
  const params = new URLSearchParams()
  if (options.path !== undefined) params.set('path', options.path)
  if (options.sort) params.set('sort', options.sort)
  if (options.order) params.set('order', options.order)

  const url = `${API_BASE_URL}/folders${params.toString() ? `?${params.toString()}` : ''}`
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      Accept: 'application/json',
    },
    signal,
  })

  if (!response.ok) {
    const errorBody = await response.json().catch(() => null)
    throw new Error(errorBody?.message || `Failed to fetch folders: HTTP ${response.status}`)
  }

  const data = await response.json()
  const folders = Array.isArray(data.folders) ? data.folders : []
  return {
    path: data.path || '',
    folders: folders,
    total: typeof data.total === 'number' ? data.total : folders.length,
  }
}

export interface FetchFolderContentsOptions {
  limit?: number
  offset?: number
  sort?: string
  order?: string
}

export async function fetchFolderContents(
  path?: string,
  options: FetchFolderContentsOptions = {},
  signal?: AbortSignal
): Promise<FolderContents> {
  const params = new URLSearchParams()
  if (path !== undefined) params.set('path', path)
  if (options.limit !== undefined) params.set('limit', options.limit.toString())
  if (options.offset !== undefined) params.set('offset', options.offset.toString())
  if (options.sort) params.set('sort', options.sort)
  if (options.order) params.set('order', options.order)

  const url = `${API_BASE_URL}/folders/contents${params.toString() ? `?${params.toString()}` : ''}`
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      Accept: 'application/json',
    },
    signal,
  })

  if (!response.ok) {
    const errorBody = await response.json().catch(() => null)
    throw new Error(errorBody?.message || `Failed to fetch folder contents: HTTP ${response.status}`)
  }

  const data = await response.json()
  return {
    ...data,
    folders: Array.isArray(data.folders) ? data.folders : [],
    breadcrumbs: Array.isArray(data.breadcrumbs) ? data.breadcrumbs : [],
    photos: (Array.isArray(data.photos) ? data.photos : []).map(toFrontendPhoto),
  }
}

export interface FetchFolderPhotosOptions {
  limit?: number
  offset?: number
  sort?: string
  order?: string
}

export async function fetchFolderPhotos(
  path: string,
  options: FetchFolderPhotosOptions = {},
  signal?: AbortSignal
): Promise<{ path: string; photos: Photo[]; total: number; limit: number; offset: number }> {
  const params = new URLSearchParams()
  params.set('path', path)
  if (options.limit !== undefined) params.set('limit', options.limit.toString())
  if (options.offset !== undefined) params.set('offset', options.offset.toString())
  if (options.sort) params.set('sort', options.sort)
  if (options.order) params.set('order', options.order)

  const url = `${API_BASE_URL}/folders/photos?${params.toString()}`
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      Accept: 'application/json',
    },
    signal,
  })

  if (!response.ok) {
    const errorBody = await response.json().catch(() => null)
    throw new Error(errorBody?.message || `Failed to fetch folder photos: HTTP ${response.status}`)
  }

  const data = await response.json()
  const photos = (Array.isArray(data.photos) ? data.photos : []).map(toFrontendPhoto)
  return {
    path: data.path || path,
    photos: photos,
    total: typeof data.total === 'number' ? data.total : photos.length,
    limit: data.limit || 50,
    offset: data.offset || 0,
  }
}
