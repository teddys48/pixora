import { Photo } from './photo'

// Virtual Album interface (stored in SQLite)
export interface Folder {
  id: string
  name: string
  description?: string
  photoCount: number
  coverUrl?: string
  createdAt: string
  updatedAt: string
}

// Physical Folder interface (discovered from filesystem under STORAGE_ROOT)
export interface PhysicalFolder {
  name: string
  path: string
  photoCount: number
  directPhotoCount: number
  subfolderCount: number
  coverPhotoId?: string
  coverThumbnailUrl?: string
  hasUnindexedFiles: boolean
}

// Breadcrumb navigation item
export interface BreadcrumbItem {
  name: string
  path: string
}

// Full contents of a physical directory
export interface FolderContents {
  name: string
  path: string
  parentPath: string | null
  breadcrumbs: BreadcrumbItem[]
  folders: PhysicalFolder[]
  photos: Photo[]
  totalPhotos: number
  limit: number
  offset: number
  diskMediaCount: number
  unindexedCount: number
  hasUnindexedFiles: boolean
}
