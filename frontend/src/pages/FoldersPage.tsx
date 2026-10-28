import { useState, useEffect, useCallback } from 'react'
import { Folder } from '@/types/folder'
import { FolderCard } from '@/components/folders/FolderCard'
import { CreateFolderModal } from '@/components/folders/CreateFolderModal'
import { Button } from '@/components/ui/Button'
import { fetchAlbums, createAlbum } from '@/lib/api'
import { FolderPlus, FolderOpen, Info, AlertCircle } from 'lucide-react'

interface FoldersPageProps {
  onSelectFolder: (folder: Folder) => void
}

export function FoldersPage({ onSelectFolder }: FoldersPageProps) {
  const [folders, setFolders] = useState<Folder[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isCreateOpen, setIsCreateOpen] = useState(false)

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
    try {
      const newAlbum = await createAlbum(name, description)
      setFolders((prev) => [newAlbum, ...prev])
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to create album'
      setError(msg)
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
            <span className="text-xs font-medium text-primary bg-primary/10 px-2.5 py-1 rounded-full border border-primary/20">
              Phase 4 Preview
            </span>
          </div>
          <p className="text-sm text-muted-foreground mt-1">
            Group your photographs into structured albums and collections.
          </p>
        </div>

        <Button
          onClick={() => setIsCreateOpen(true)}
          size="sm"
          className="gap-2 self-start sm:self-auto h-9 font-medium"
        >
          <FolderPlus className="w-4 h-4" />
          <span>New Folder</span>
        </Button>
      </div>

      {/* Phase 4 Notice Banner */}
      <div className="flex items-start gap-3 p-3.5 rounded-xl border border-border/70 bg-card/60 text-xs text-muted-foreground">
        <Info className="w-4 h-4 text-primary shrink-0 mt-0.5" />
        <div className="space-y-1">
          <p className="font-medium text-foreground">Album Management Status</p>
          <p className="leading-relaxed">
            Persistent album creation is enabled via the SQLite database backend. Full album management—including
            assigning photos to albums, setting album cover photos, and organizing filesystem paths—is planned for{' '}
            <strong>Phase 4</strong>.
          </p>
        </div>
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

      {/* Folders Grid */}
      {isLoading ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 md:gap-6 animate-pulse">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-48 rounded-xl bg-muted/40 border border-border/60" />
          ))}
        </div>
      ) : folders.length === 0 ? (
        <div className="flex flex-col items-center justify-center p-14 text-center border border-dashed border-border rounded-2xl bg-card/40 my-8">
          <div className="flex items-center justify-center w-14 h-14 rounded-full bg-primary/10 text-primary mb-4">
            <FolderOpen className="w-7 h-7 stroke-[1.5]" />
          </div>
          <h3 className="text-lg font-semibold text-foreground mb-1">No albums created yet</h3>
          <p className="text-sm text-muted-foreground max-w-sm mb-5 leading-relaxed">
            Create your first album folder to organize your photos. Album creation is saved directly to your local database.
          </p>
          <Button onClick={() => setIsCreateOpen(true)} size="md" className="gap-2">
            <FolderPlus className="w-4 h-4" />
            <span>Create Folder</span>
          </Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 md:gap-6">
          {folders.map((folder) => (
            <FolderCard
              key={folder.id}
              folder={folder}
              onClick={onSelectFolder}
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
    </div>
  )
}
