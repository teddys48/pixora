import { useState, useEffect, useCallback, useMemo } from 'react'
import { useParams, useNavigate, useSearchParams } from 'react-router-dom'
import { Folder, PhysicalFolder } from '@/types/folder'
import { Photo } from '@/types/photo'
import { FolderCard } from '@/components/folders/FolderCard'
import { PhysicalFolderCard } from '@/components/folders/PhysicalFolderCard'
import { PhysicalFolderDetailView } from '@/components/folders/PhysicalFolderDetailView'
import { CreateFolderModal } from '@/components/folders/CreateFolderModal'
import { RenameFolderModal } from '@/components/folders/RenameFolderModal'
import { DeleteFolderModal } from '@/components/folders/DeleteFolderModal'
import { AlbumDetailView } from '@/components/folders/AlbumDetailView'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import {
  fetchAlbums,
  createAlbum,
  updateAlbum,
  deleteAlbum,
  fetchPhysicalFolders,
  triggerScan,
} from '@/lib/api'
import {
  FolderPlus,
  FolderOpen,
  AlertCircle,
  Search,
  ArrowUpDown,
  HardDrive,
  RefreshCw,
  CheckCircle2,
} from 'lucide-react'

interface FoldersPageProps {
  onSelectFolder?: (folder: Folder) => void
  onSelectPhoto?: (photo: Photo, photosList: Photo[]) => void
}

type AlbumSortOption = 'name-asc' | 'name-desc' | 'photos-desc' | 'updated-desc'
type PhysicalSortOption = 'name-asc' | 'name-desc' | 'photos-desc' | 'subfolders-desc'

export function FoldersPage({ onSelectPhoto }: FoldersPageProps) {
  const { albumId } = useParams<{ albumId?: string }>()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()

  const activeTab = (searchParams.get('tab') as 'physical' | 'albums') || 'physical'
  const currentPhysicalPath = searchParams.get('path') || ''

  // Virtual Albums State
  const [folders, setFolders] = useState<Folder[]>([])
  const [isLoadingAlbums, setIsLoadingAlbums] = useState(true)
  const [albumError, setAlbumError] = useState<string | null>(null)
  const [albumSearchQuery, setAlbumSearchQuery] = useState('')
  const [albumSortOption, setAlbumSortOption] = useState<AlbumSortOption>('name-asc')

  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [editingFolder, setEditingFolder] = useState<Folder | null>(null)
  const [deletingFolder, setDeletingFolder] = useState<Folder | null>(null)

  // Physical Folders State
  const [physicalFolders, setPhysicalFolders] = useState<PhysicalFolder[]>([])
  const [isLoadingPhysical, setIsLoadingPhysical] = useState(true)
  const [physicalError, setPhysicalError] = useState<string | null>(null)
  const [physicalSearchQuery, setPhysicalSearchQuery] = useState('')
  const [physicalSortOption, setPhysicalSortOption] = useState<PhysicalSortOption>('name-asc')
  const [isScanning, setIsScanning] = useState(false)
  const [scanMessage, setScanMessage] = useState<string | null>(null)

  const loadAlbums = useCallback(async () => {
    setIsLoadingAlbums(true)
    setAlbumError(null)
    try {
      const albums = await fetchAlbums()
      setFolders(albums)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load albums'
      setAlbumError(msg)
    } finally {
      setIsLoadingAlbums(false)
    }
  }, [])

  const loadPhysicalFolders = useCallback(async () => {
    setIsLoadingPhysical(true)
    setPhysicalError(null)
    try {
      const data = await fetchPhysicalFolders()
      setPhysicalFolders(data.folders || [])
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load physical folders'
      setPhysicalError(msg)
    } finally {
      setIsLoadingPhysical(false)
    }
  }, [])

  useEffect(() => {
    loadAlbums()
    loadPhysicalFolders()
  }, [loadAlbums, loadPhysicalFolders])

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
    setDeletingFolder(null)
    if (albumId) {
      navigate('/folders?tab=albums')
    }
  }

  const handleUpdateAlbumFromDetail = (updated: Folder) => {
    setFolders((prev) => prev.map((f) => (f.id === updated.id ? updated : f)))
  }

  const handleTabChange = (tab: 'physical' | 'albums') => {
    const nextParams = new URLSearchParams()
    nextParams.set('tab', tab)
    setSearchParams(nextParams)
  }

  const handleNavigatePhysical = (newPath: string) => {
    const nextParams = new URLSearchParams(searchParams)
    nextParams.set('tab', 'physical')
    if (newPath) {
      nextParams.set('path', newPath)
    } else {
      nextParams.delete('path')
    }
    setSearchParams(nextParams)
  }

  const handleScanLibrary = async () => {
    setIsScanning(true)
    setPhysicalError(null)
    setScanMessage(null)
    try {
      const status = await triggerScan(true)
      const indexed = status.indexed || 0
      setScanMessage(
        indexed > 0
          ? `Media scan completed: Indexed ${indexed} new photo${indexed === 1 ? '' : 's'}.`
          : 'Media scan completed: All media files in /media are up to date.'
      )
      await loadPhysicalFolders()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to scan library'
      setPhysicalError(msg)
    } finally {
      setIsScanning(false)
    }
  }

  const filteredAndSortedAlbums = useMemo(() => {
    let result = [...folders]

    if (albumSearchQuery.trim()) {
      const q = albumSearchQuery.toLowerCase().trim()
      result = result.filter(
        (f) =>
          f.name.toLowerCase().includes(q) ||
          (f.description && f.description.toLowerCase().includes(q))
      )
    }

    result.sort((a, b) => {
      switch (albumSortOption) {
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
  }, [folders, albumSearchQuery, albumSortOption])

  const filteredAndSortedPhysical = useMemo(() => {
    let result = [...physicalFolders]

    if (physicalSearchQuery.trim()) {
      const q = physicalSearchQuery.toLowerCase().trim()
      result = result.filter(
        (f) =>
          f.name.toLowerCase().includes(q) ||
          f.path.toLowerCase().includes(q)
      )
    }

    result.sort((a, b) => {
      switch (physicalSortOption) {
        case 'name-asc':
          return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
        case 'name-desc':
          return b.name.localeCompare(a.name, undefined, { sensitivity: 'base' })
        case 'photos-desc':
          return b.photoCount - a.photoCount
        case 'subfolders-desc':
          return b.subfolderCount - a.subfolderCount
        default:
          return 0
      }
    })

    return result
  }, [physicalFolders, physicalSearchQuery, physicalSortOption])

  const currentAlbum = useMemo(() => {
    if (!albumId) return null
    return folders.find((f) => f.id === albumId) || null
  }, [albumId, folders])

  if (albumId) {
    if (isLoadingAlbums && !currentAlbum) {
      return (
        <div className="flex flex-col items-center justify-center p-20 animate-pulse text-muted-foreground">
          <div className="w-10 h-10 rounded-full bg-muted/60 mb-3" />
          <p className="text-sm">Loading album...</p>
        </div>
      )
    }

    if (!currentAlbum && !isLoadingAlbums) {
      return (
        <div className="flex flex-col items-center justify-center p-14 text-center border border-dashed border-border rounded-2xl bg-card/40 my-8">
          <AlertCircle className="w-10 h-10 text-destructive mb-3" />
          <h3 className="text-lg font-semibold text-foreground mb-1">Album not found</h3>
          <p className="text-sm text-muted-foreground max-w-sm mb-5 leading-relaxed">
            The requested album could not be found or may have been deleted.
          </p>
          <Button onClick={() => navigate('/folders?tab=albums')} size="sm">
            &larr; Back to all albums
          </Button>
        </div>
      )
    }

    if (currentAlbum) {
      return (
        <AlbumDetailView
          album={currentAlbum}
          onBack={() => navigate('/folders?tab=albums')}
          onSelectPhoto={onSelectPhoto || (() => {})}
          onUpdateAlbum={handleUpdateAlbumFromDetail}
          onDeleteAlbum={handleDeleteFolder}
        />
      )
    }
  }

  if (activeTab === 'physical' && currentPhysicalPath) {
    return (
      <PhysicalFolderDetailView
        path={currentPhysicalPath}
        onNavigate={handleNavigatePhysical}
        onSelectPhoto={onSelectPhoto || (() => {})}
      />
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2.5">
            <h1 className="text-2xl font-bold tracking-tight text-foreground">
              Folders & Albums
            </h1>
            <span className="text-xs font-normal text-muted-foreground bg-muted px-2.5 py-0.5 rounded-full border border-border/60">
              {activeTab === 'physical'
                ? `${physicalFolders.length} top-level ${physicalFolders.length === 1 ? 'folder' : 'folders'}`
                : `${folders.length} ${folders.length === 1 ? 'album' : 'albums'}`}
            </span>
          </div>
          <p className="text-sm text-muted-foreground mt-1">
            {activeTab === 'physical'
              ? 'Browse physical directories directly from your mounted media storage (/media).'
              : 'Organize your photographs into virtual collections without modifying or moving original files.'}
          </p>
        </div>

        {activeTab === 'physical' ? (
          <Button
            onClick={handleScanLibrary}
            disabled={isScanning}
            variant="outline"
            size="sm"
            className="gap-2 self-start sm:self-auto h-9 font-medium text-xs"
            title="Scan /media for newly added or modified photos"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${isScanning ? 'animate-spin' : ''}`} />
            <span>{isScanning ? 'Scanning...' : 'Scan Media Library'}</span>
          </Button>
        ) : (
          <Button
            onClick={() => setIsCreateOpen(true)}
            size="sm"
            className="gap-2 self-start sm:self-auto h-9 font-medium text-xs"
          >
            <FolderPlus className="w-4 h-4" />
            <span>New Album</span>
          </Button>
        )}
      </div>

      <div className="flex items-center justify-between flex-wrap gap-3 pb-1 border-b border-border/60">
        <div className="inline-flex items-center p-1 bg-muted/60 rounded-xl border border-border/60">
          <button
            type="button"
            onClick={() => handleTabChange('physical')}
            className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-medium transition-all ${
              activeTab === 'physical'
                ? 'bg-background text-foreground shadow-xs border border-border/40 font-semibold'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            <HardDrive className="w-3.5 h-3.5 text-primary" />
            <span>Physical Folders</span>
            <span className="text-[10px] font-mono px-1.5 py-0.2 rounded bg-muted/80 text-muted-foreground border border-border/40">
              /media
            </span>
          </button>
          <button
            type="button"
            onClick={() => handleTabChange('albums')}
            className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-medium transition-all ${
              activeTab === 'albums'
                ? 'bg-background text-foreground shadow-xs border border-border/40 font-semibold'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            <FolderOpen className="w-3.5 h-3.5 text-primary" />
            <span>Virtual Albums</span>
            <span className="text-[10px] font-mono px-1.5 py-0.2 rounded bg-muted/80 text-muted-foreground border border-border/40">
              {folders.length}
            </span>
          </button>
        </div>
      </div>

      {activeTab === 'physical' && (
        <div className="space-y-6">
          {physicalError && (
            <div className="flex items-center justify-between p-3.5 rounded-xl border border-destructive/20 bg-destructive/10 text-destructive text-xs">
              <div className="flex items-center gap-2">
                <AlertCircle className="w-4 h-4 shrink-0" />
                <span>{physicalError}</span>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={loadPhysicalFolders}
                className="h-7 text-xs ml-3"
              >
                Retry
              </Button>
            </div>
          )}

          {scanMessage && (
            <div className="flex items-center justify-between p-3.5 rounded-xl border border-emerald-500/20 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 text-xs animate-in fade-in">
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

          {physicalFolders.length > 0 && (
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 p-2 rounded-xl bg-card border border-border/70 text-xs">
              <div className="relative flex-1 max-w-sm">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground" />
                <Input
                  type="text"
                  placeholder="Search physical folders..."
                  value={physicalSearchQuery}
                  onChange={(e) => setPhysicalSearchQuery(e.target.value)}
                  className="pl-8 h-8 text-xs border-transparent bg-transparent focus-visible:bg-background"
                />
              </div>

              <div className="flex items-center gap-2 self-end sm:self-auto">
                <ArrowUpDown className="w-3.5 h-3.5 text-muted-foreground" />
                <select
                  value={physicalSortOption}
                  onChange={(e) => setPhysicalSortOption(e.target.value as PhysicalSortOption)}
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
                  <option value="subfolders-desc" className="bg-popover text-popover-foreground">
                    Most Subfolders
                  </option>
                </select>
              </div>
            </div>
          )}

          {isLoadingPhysical ? (
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 md:gap-6 animate-pulse">
              {[1, 2, 3, 4].map((i) => (
                <div key={i} className="h-56 rounded-xl bg-muted/40 border border-border/60" />
              ))}
            </div>
          ) : physicalFolders.length === 0 ? (
            <div className="flex flex-col items-center justify-center p-14 text-center border border-dashed border-border rounded-2xl bg-card/40 my-8">
              <div className="flex items-center justify-center w-14 h-14 rounded-full bg-primary/10 text-primary mb-4">
                <HardDrive className="w-7 h-7 stroke-[1.5]" />
              </div>
              <h3 className="text-lg font-semibold text-foreground mb-1">
                No physical folders found in /media
              </h3>
              <p className="text-sm text-muted-foreground max-w-md mb-5 leading-relaxed">
                Pixora is configured to browse media mounted at <code className="px-1.5 py-0.5 rounded bg-muted font-mono text-xs">/media</code>. If you just placed directories or pictures there, trigger a media scan to index them.
              </p>
              <Button
                onClick={handleScanLibrary}
                disabled={isScanning}
                size="md"
                className="gap-2"
              >
                <RefreshCw className={`w-4 h-4 ${isScanning ? 'animate-spin' : ''}`} />
                <span>{isScanning ? 'Scanning Library...' : 'Scan Media Library'}</span>
              </Button>
            </div>
          ) : filteredAndSortedPhysical.length === 0 ? (
            <div className="flex flex-col items-center justify-center p-12 text-center border border-dashed border-border rounded-2xl bg-card/40">
              <p className="text-sm font-medium text-foreground mb-1">No matching folders found</p>
              <p className="text-xs text-muted-foreground mb-4">
                No physical folder matched &quot;{physicalSearchQuery}&quot;.
              </p>
              <Button variant="outline" size="sm" onClick={() => setPhysicalSearchQuery('')}>
                Clear Search
              </Button>
            </div>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 md:gap-6">
              {filteredAndSortedPhysical.map((folder) => (
                <PhysicalFolderCard
                  key={folder.path}
                  folder={folder}
                  onClick={(f) => handleNavigatePhysical(f.path)}
                />
              ))}
            </div>
          )}
        </div>
      )}

      {activeTab === 'albums' && (
        <div className="space-y-6">
          {albumError && (
            <div className="flex items-center justify-between p-3.5 rounded-xl border border-destructive/20 bg-destructive/10 text-destructive text-xs">
              <div className="flex items-center gap-2">
                <AlertCircle className="w-4 h-4 shrink-0" />
                <span>{albumError}</span>
              </div>
              <Button variant="outline" size="sm" onClick={loadAlbums} className="h-7 text-xs ml-3">
                Retry
              </Button>
            </div>
          )}

          {folders.length > 0 && (
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 p-2 rounded-xl bg-card border border-border/70 text-xs">
              <div className="relative flex-1 max-w-sm">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground" />
                <Input
                  type="text"
                  placeholder="Search albums..."
                  value={albumSearchQuery}
                  onChange={(e) => setAlbumSearchQuery(e.target.value)}
                  className="pl-8 h-8 text-xs border-transparent bg-transparent focus-visible:bg-background"
                />
              </div>

              <div className="flex items-center gap-2 self-end sm:self-auto">
                <ArrowUpDown className="w-3.5 h-3.5 text-muted-foreground" />
                <select
                  value={albumSortOption}
                  onChange={(e) => setAlbumSortOption(e.target.value as AlbumSortOption)}
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

          {isLoadingAlbums ? (
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
          ) : filteredAndSortedAlbums.length === 0 ? (
            <div className="flex flex-col items-center justify-center p-12 text-center border border-dashed border-border rounded-2xl bg-card/40">
              <p className="text-sm font-medium text-foreground mb-1">No matching albums found</p>
              <p className="text-xs text-muted-foreground mb-4">
                No album matched &quot;{albumSearchQuery}&quot;.
              </p>
              <Button variant="outline" size="sm" onClick={() => setAlbumSearchQuery('')}>
                Clear Search
              </Button>
            </div>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 md:gap-6">
              {filteredAndSortedAlbums.map((folder) => (
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
        </div>
      )}

      <CreateFolderModal
        isOpen={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
        onCreate={handleCreateFolder}
      />

      <RenameFolderModal
        isOpen={Boolean(editingFolder)}
        folder={editingFolder}
        onClose={() => setEditingFolder(null)}
        onRename={handleRenameFolder}
      />

      <DeleteFolderModal
        isOpen={Boolean(deletingFolder)}
        folder={deletingFolder}
        onClose={() => setDeletingFolder(null)}
        onConfirm={handleDeleteFolder}
      />
    </div>
  )
}
