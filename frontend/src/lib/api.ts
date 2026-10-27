import { HealthResponse } from '@/types/health'
import {
  Photo,
  PhotoResponse,
  PhotoListResponse,
  ScanStatus,
  UploadResponse,
} from '@/types/photo'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api'

export function toFrontendPhoto(p: PhotoResponse): Photo {
  const date = p.takenAt || p.fileModifiedAt || p.createdAt
  // Clean filename without extension for title
  const title = p.originalFilename.replace(/\.[^/.]+$/, '')
  return {
    id: p.id,
    title: title,
    filename: p.originalFilename,
    url: p.url,
    thumbnailUrl: p.thumbnailUrl,
    date: date,
    size: p.fileSize,
    metadata: {
      width: p.width || 0,
      height: p.height || 0,
    },
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

export async function fetchPhoto(id: string, signal?: AbortSignal): Promise<Photo> {
  const response = await fetch(`${API_BASE_URL}/photos/${encodeURIComponent(id)}`, {
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

export interface UploadResult {
  photos: Photo[]
  errors?: { filename: string; error: string }[]
}

export function uploadPhotos(
  files: File[],
  onProgress?: (percent: number) => void
): Promise<UploadResult> {
  return new Promise((resolve, reject) => {
    const formData = new FormData()
    files.forEach((file) => {
      formData.append('photos', file)
    })

    const xhr = new XMLHttpRequest()
    xhr.open('POST', `${API_BASE_URL}/photos`)
    xhr.setRequestHeader('Accept', 'application/json')

    if (xhr.upload && onProgress) {
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) {
          const percent = Math.round((e.loaded / e.total) * 100)
          onProgress(percent)
        }
      }
    }

    xhr.onload = () => {
      let data: UploadResponse
      try {
        data = JSON.parse(xhr.responseText)
      } catch {
        return reject(new Error(`Failed to parse server upload response (HTTP ${xhr.status})`))
      }

      if (xhr.status >= 200 && xhr.status < 300) {
        resolve({
          photos: (data.photos || []).map(toFrontendPhoto),
          errors: data.errors,
        })
      } else if (xhr.status === 207) {
        // Multi-status (partial success)
        resolve({
          photos: (data.photos || []).map(toFrontendPhoto),
          errors: data.errors,
        })
      } else {
        const msg = data.message || `Upload failed with HTTP ${xhr.status}`
        reject(new Error(msg))
      }
    }

    xhr.onerror = () => {
      reject(new Error('Network error occurred during photo upload'))
    }

    xhr.send(formData)
  })
}

export async function triggerScan(sync = false): Promise<ScanStatus> {
  const url = `${API_BASE_URL}/photos/scan${sync ? '?sync=true' : ''}`
  const response = await fetch(url, {
    method: 'POST',
    headers: {
      Accept: 'application/json',
    },
  })

  const data: ScanStatus = await response.json().catch(() => ({
    status: 'failed',
    scanned: 0,
    indexed: 0,
    updated: 0,
    skipped: 0,
    failed: 1,
    errors: [`HTTP ${response.status}`],
  }))

  return data
}

export async function fetchScanStatus(): Promise<ScanStatus> {
  const response = await fetch(`${API_BASE_URL}/photos/scan`, {
    method: 'GET',
    headers: {
      Accept: 'application/json',
    },
  })

  if (!response.ok) {
    throw new Error(`Failed to get scan status: HTTP ${response.status}`)
  }

  return response.json()
}
