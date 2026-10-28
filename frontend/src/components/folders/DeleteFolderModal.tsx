import { useState } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Folder } from '@/types/folder'
import { Trash2, AlertTriangle, Loader2, ShieldCheck } from 'lucide-react'

interface DeleteFolderModalProps {
  isOpen: boolean
  folder: Folder | null
  onClose: () => void
  onConfirm: (id: string) => Promise<void>
}

export function DeleteFolderModal({
  isOpen,
  folder,
  onClose,
  onConfirm,
}: DeleteFolderModalProps) {
  const [isDeleting, setIsDeleting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleClose = () => {
    if (isDeleting) return
    setError(null)
    onClose()
  }

  const handleDelete = async () => {
    if (!folder) return
    setIsDeleting(true)
    setError(null)
    try {
      await onConfirm(folder.id)
      onClose()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to delete album'
      setError(msg)
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title="Delete Album"
      description={`Are you sure you want to delete the album "${folder?.name || ''}"?`}
      maxWidth="md"
    >
      <div className="space-y-4">
        {error && (
          <div className="flex items-center gap-2 p-3 rounded-lg bg-destructive/10 border border-destructive/20 text-destructive text-xs">
            <AlertTriangle className="w-4 h-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        <div className="p-3.5 rounded-xl border border-border/80 bg-muted/40 space-y-2 text-xs text-muted-foreground">
          <div className="flex items-center gap-2 text-foreground font-medium">
            <ShieldCheck className="w-4 h-4 text-emerald-500 shrink-0" />
            <span>Original Media Files are Protected</span>
          </div>
          <p className="leading-relaxed">
            Deleting this virtual album removes only the collection and photo groupings in the database.
            None of the <strong>{folder?.photoCount || 0} photo{folder?.photoCount === 1 ? '' : 's'}</strong> in this album will be deleted from your photo library or storage disk.
          </p>
        </div>

        <div className="flex items-center justify-end gap-2 pt-3 border-t border-border">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={handleClose}
            disabled={isDeleting}
          >
            Cancel
          </Button>
          <Button
            type="button"
            variant="destructive"
            size="sm"
            onClick={handleDelete}
            disabled={isDeleting}
            className="gap-1.5"
          >
            {isDeleting ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Trash2 className="w-4 h-4" />
            )}
            <span>{isDeleting ? 'Deleting...' : 'Delete Album'}</span>
          </Button>
        </div>
      </div>
    </Modal>
  )
}
