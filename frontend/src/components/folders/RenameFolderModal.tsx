import React, { useState, useEffect } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Folder } from '@/types/folder'
import { Edit2, AlertCircle, Loader2 } from 'lucide-react'

interface RenameFolderModalProps {
  isOpen: boolean
  folder: Folder | null
  onClose: () => void
  onRename: (id: string, newName: string, newDescription: string) => Promise<void>
}

export function RenameFolderModal({
  isOpen,
  folder,
  onClose,
  onRename,
}: RenameFolderModalProps) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (folder) {
      setName(folder.name)
      setDescription(folder.description || '')
      setError(null)
    }
  }, [folder, isOpen])

  const handleClose = () => {
    if (isSubmitting) return
    setError(null)
    onClose()
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!folder) return

    const trimmedName = name.trim()
    if (!trimmedName) {
      setError('Album name cannot be empty.')
      return
    }
    if (trimmedName.length > 100) {
      setError('Album name cannot exceed 100 characters.')
      return
    }

    setIsSubmitting(true)
    setError(null)
    try {
      await onRename(folder.id, trimmedName, description.trim())
      onClose()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to rename album'
      setError(msg)
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title="Rename Album"
      description={`Update the name or description for "${folder?.name || ''}".`}
      maxWidth="md"
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && (
          <div className="flex items-center gap-2 p-3 rounded-lg bg-destructive/10 border border-destructive/20 text-destructive text-xs">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        <div>
          <label htmlFor="rename-folder-name" className="block text-xs font-medium text-foreground mb-1.5">
            Album Name <span className="text-destructive">*</span>
          </label>
          <Input
            id="rename-folder-name"
            value={name}
            onChange={(e) => {
              setName(e.target.value)
              if (error) setError(null)
            }}
            placeholder="Album name"
            maxLength={100}
            required
            autoFocus
            disabled={isSubmitting}
          />
          <div className="flex justify-between items-center mt-1 text-[11px] text-muted-foreground">
            <span>Name must be unique</span>
            <span>{name.length}/100</span>
          </div>
        </div>

        <div>
          <label htmlFor="rename-folder-desc" className="block text-xs font-medium text-foreground mb-1.5">
            Description (Optional)
          </label>
          <Input
            id="rename-folder-desc"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Brief summary of photos in this album"
            maxLength={500}
            disabled={isSubmitting}
          />
        </div>

        <div className="flex items-center justify-end gap-2 pt-3 border-t border-border">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={handleClose}
            disabled={isSubmitting}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            size="sm"
            disabled={!name.trim() || isSubmitting}
            className="gap-1.5"
          >
            {isSubmitting ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Edit2 className="w-4 h-4" />
            )}
            <span>{isSubmitting ? 'Saving...' : 'Save Changes'}</span>
          </Button>
        </div>
      </form>
    </Modal>
  )
}
