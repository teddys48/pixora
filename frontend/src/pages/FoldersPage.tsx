import { useState, useEffect, useCallback, useMemo } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Folder } from '@/types/folder'
import { Photo } from '@/types/photo'
import { FolderCard } from '@/components/folders/FolderCard'
import { CreateFolderModal } from '@/components/folders/CreateFolderModal'
import { RenameFolderModal } from '@/components/folders/RenameFolderModal'
import { DeleteFolderModal } from '@/components/folders/DeleteFolderModal'
import { AlbumDetailView } from '@/components/folders/AlbumDetailView'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { fetchAlbums, createAlbum, updateAlbum, deleteAlbum } from '@/lib/api'
import {
  FolderPlus,
  FolderOpen,
  AlertCircle,
  Search,
  ArrowUpDown,
} from 'lucide-react'

interface FoldersPageProps {
  onSelectFolder?: (folder: Folder) => void
  onSelectPhoto?: (photo: Photo, photosList: Photo[]) => void
}

type AlbumSortOption = 'name-asc' | 'name-desc' | 'photos-desc' | 'updated-desc'

export function FoldersPage({ onSelectPhoto }: FoldersPageProps) {
  const { albumId } = useParams<{ albumId?: string }>()
  const navigate = useNavigate()

  const [folders, setFolders] = useState<Folder[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [searchQuery, setSearchQuery] = useState('')
  const [sortOption, setSortOption] = useState<AlbumSortOption>('name-asc')

  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [editingFolder, setEditingFolder] = useState<Folder | null>(null)
  const [deletingFolder, setDeletingFolder] = useState<Folder | null>(null)

  const loadAlbums = useCallback(async () => {
    setIsLoading(true)
    setError(null)
    try {
      const albums = await fetchAlbums()
      setFolders(albums)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load albums'
      setError(msg)
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    loadAlbums()
  }, [loadAlbums])

  const handleCreateFolder = async (name: string, description: string) => {
    const newAlbum = await createAlbum(name, description)
    setFolders((prev) => [newAlbum, ...prev])
  }

  const handleRenameFolder = async (id: string, newName: string, newDescription: string) => {
    const updated = await updateAlbum(id, newName, newDescription)
    setFolders((prev) => prev.map((f) => (f.id === id ? updated : f)))
  }

  const handleDeleteFolder = async (id: string) => {
    await deleteAlbum(id)
    setFolders((prev) => prev.filter((f) => f.id !== id))
    if (albumId === id) {
      navigate('/folders')
    }
  }

  const handleUpdateAlbumFromDetail = (updated: Folder) => {
    setFolders((prev) => prev.map((f) => (f.id === updated.id ? updated : f)))
  }

  const filteredAndSortedFolders = useMemo(() => {
    let result = [...folders]

    // Search filter
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase().trim()
      result = result.filter(
        (f) =>
          f.name.toLowerCase().includes(q) ||
          (f.description && f.description.toLowerCase().includes(q))
      )
    }

    // Sort
    result.sort((a, b) => {
      switch (sortOption) {
        case 'name-asc':
          return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
        case 'name-desc':
          return b.name.localeCompare(a.name, undefined, { sensitivity: 'base' })
        case 'photos-desc':
          return b.photoCount - a.photoCount
        case 'updated-desc':
          return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
        default:
          return 0
      }
    })

    return result
  }, [folders, searchQuery, sortOption])

  // If albumId is present in URL, find the album and show the detail view
  const currentAlbum = useMemo(() => {
    if (!albumId) return null
    return folders.find((f) => f.id === albumId) || null
  }, [albumId, folders])

  if (albumId) {
    if (isLoading && !currentAlbum) {
      return (
        <div className="flex flex-col items-center justify-center p-20 animate-pulse text-muted-foreground">
          <div className="w-10 h-10 rounded-full bg-muted/60 mb-3" />
          <p className="text-sm">Loading album...</p>
        </div>
      )
    }

    if (!currentAlbum && !isLoading) {
      return (
        <div className="flex flex-col items-center justify-center p-14 text-center border border-dashed border-border rounded-2xl bg-card/40 my-8">
          <AlertCircle className="w-10 h-10 text-destructive mb-3" />
          <h3 className="text-lg font-semibold text-foreground mb-1">Album not found</h3>
          <p className="text-sm text-muted-foreground max-w-sm mb-5 leading-relaxed">
            The requested album could not be found or may have been deleted.
          </p>
          <Button onClick={() => navigate('/folders')} size="sm">
            &larr; Back to all albums
          </Button>
        </div>
      )
    }

    if (currentAlbum) {
      return (
        <AlbumDetailView
          album={currentAlbum}
          onBack={() => navigate('/folders')}
          onSelectPhoto={onSelectPhoto || (() => {})}
          onUpdateAlbum={handleUpdateAlbumFromDetail}
          onDeleteAlbum={handleDeleteFolder}
        />
      )
    }
  }

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-2xl font-bold tracking-tight text-foreground">
              Albums & Folders
            </h1>
            <span className="text-xs font-normal text-muted-foreground bg-muted px-2 py-0.5 rounded-full border border-border/60">
              {folders.length} {folders.length === 1 ? 'collection' : 'collections'}
            </span>
          </div>
          <p className="text-sm text-muted-foreground mt-1">
            Group your photographs into virtual collections without modifying or moving original files.
          </p>
        </div>

        <Button
          onClick={() => setIsCreateOpen(true)}
          size="sm"
          className="gap-2 self-start sm:self-auto h-9 font-medium"
        >
          <FolderPlus className="w-4 h-4" />
          <span>New Album</span>
        </Button>
      </div>

      {/* Error Alert */}
      {error && (
        <div className="flex items-center justify-between p-3.5 rounded-xl border border-destructive/20 bg-destructive/10 text-destructive text-xs">
          <div className="flex items-center gap-2">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{error}</span>
          </div>
          <Button variant="outline" size="sm" onClick={loadAlbums} className="h-7 text-xs ml-3">
            Retry
          </Button>
        </div>
      )}

      {/* Search and Sort Toolbar */}
      {folders.length > 0 && (
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 p-2 rounded-xl bg-card border border-border/70 text-xs">
          <div className="relative flex-1 max-w-sm">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground" />
            <Input
              type="text"
              placeholder="Search albums..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-8 h-8 text-xs border-transparent bg-transparent focus-visible:bg-background"
            />
          </div>

          <div className="flex items-center gap-2 self-end sm:self-auto">
            <ArrowUpDown className="w-3.5 h-3.5 text-muted-foreground" />
            <select
              value={sortOption}
              onChange={(e) => setSortOption(e.target.value as AlbumSortOption)}
              className="bg-transparent text-xs text-foreground font-medium rounded-md px-2 py-1 border border-border/60 focus:outline-none focus:ring-1 focus:ring-ring"
            >
              <option value="name-asc" className="bg-popover text-popover-foreground">
                Name (A-Z)
              </option>
              <option value="name-desc" className="bg-popover text-popover-foreground">
                Name (Z-A)
              </option>
              <option value="photos-desc" className="bg-popover text-popover-foreground">
                Most Photos
              </option>
              <option value="updated-desc" className="bg-popover text-popover-foreground">
                Recently Updated
              </option>
            </select>
          </div>
        </div>
      )}

      {/* Folders Grid */}
      {isLoading ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 md:gap-6 animate-pulse">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-56 rounded-xl bg-muted/40 border border-border/60" />
          ))}
        </div>
      ) : folders.length === 0 ? (
        <div className="flex flex-col items-center justify-center p-14 text-center border border-dashed border-border rounded-2xl bg-card/40 my-8">
          <div className="flex items-center justify-center w-14 h-14 rounded-full bg-primary/10 text-primary mb-4">
            <FolderOpen className="w-7 h-7 stroke-[1.5]" />
          </div>
          <h3 className="text-lg font-semibold text-foreground mb-1">No albums created yet</h3>
          <p className="text-sm text-muted-foreground max-w-sm mb-5 leading-relaxed">
            Create your first virtual album to organize your photos. Albums are saved directly in your database without modifying your files.
          </p>
          <Button onClick={() => setIsCreateOpen(true)} size="md" className="gap-2">
            <FolderPlus className="w-4 h-4" />
            <span>Create Album</span>
          </Button>
        </div>
      ) : filteredAndSortedFolders.length === 0 ? (
        <div className="flex flex-col items-center justify-center p-12 text-center border border-dashed border-border rounded-2xl bg-card/40">
          <p className="text-sm font-medium text-foreground mb-1">No matching albums found</p>
          <p className="text-xs text-muted-foreground mb-4">
            No album matched &quot;{searchQuery}&quot;.
          </p>
          <Button variant="outline" size="sm" onClick={() => setSearchQuery('')}>
            Clear Search
          </Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 md:gap-6">
          {filteredAndSortedFolders.map((folder) => (
            <FolderCard
              key={folder.id}
              folder={folder}
              onClick={(f) => navigate(`/folders/${f.id}`)}
              onRename={(f) => setEditingFolder(f)}
              onDelete={(f) => setDeletingFolder(f)}
            />
          ))}
        </div>
      )}

      {/* Create Folder Modal */}
      <CreateFolderModal
        isOpen={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
        onCreate={handleCreateFolder}
      />

      {/* Rename Folder Modal */}
      <RenameFolderModal
        isOpen={Boolean(editingFolder)}
        folder={editingFolder}
        onClose={() => setEditingFolder(null)}
        onRename={handleRenameFolder}
      />

      {/* Delete Folder Modal */}
      <DeleteFolderModal
        isOpen={Boolean(deletingFolder)}
        folder={deletingFolder}
        onClose={() => setDeletingFolder(null)}
        onConfirm={handleDeleteFolder}
      />
    </div>
  )
}
