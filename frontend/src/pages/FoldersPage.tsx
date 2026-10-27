import { useState } from 'react'
import { Folder } from '@/types/folder'
import { MOCK_FOLDERS } from '@/data/mock/folders'
import { FolderCard } from '@/components/folders/FolderCard'
import { CreateFolderModal } from '@/components/folders/CreateFolderModal'
import { Button } from '@/components/ui/Button'
import { FolderPlus, FolderOpen } from 'lucide-react'

interface FoldersPageProps {
  onSelectFolder: (folder: Folder) => void
}

export function FoldersPage({ onSelectFolder }: FoldersPageProps) {
  const [folders, setFolders] = useState<Folder[]>(MOCK_FOLDERS)
  const [isCreateOpen, setIsCreateOpen] = useState(false)

  const handleCreateFolder = (name: string, description: string) => {
    const newFolder: Folder = {
      id: `folder-${Date.now()}`,
      name,
      description,
      photoCount: 0,
      coverUrl: undefined,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    }
    setFolders((prev) => [newFolder, ...prev])
  }

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground flex items-center gap-2">
            <span>Albums & Folders</span>
            <span className="text-xs font-normal text-muted-foreground bg-muted px-2 py-0.5 rounded-full border border-border/60">
              {folders.length} collections
            </span>
          </h1>
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

      {/* Folders Grid */}
      {folders.length === 0 ? (
        <div className="flex flex-col items-center justify-center p-14 text-center border border-dashed border-border rounded-2xl bg-card/40 my-8">
          <div className="flex items-center justify-center w-14 h-14 rounded-full bg-primary/10 text-primary mb-4">
            <FolderOpen className="w-7 h-7 stroke-[1.5]" />
          </div>
          <h3 className="text-lg font-semibold text-foreground mb-1">No albums created yet</h3>
          <p className="text-sm text-muted-foreground max-w-sm mb-5 leading-relaxed">
            Create your first album folder to organize your photos by event, trip, or theme.
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
