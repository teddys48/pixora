import React, { useState } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { FolderPlus, AlertCircle, Loader2 } from 'lucide-react'

interface CreateFolderModalProps {
  isOpen: boolean
  onClose: () => void
  onCreate: (name: string, description: string) => Promise<void>
}

export function CreateFolderModal({ isOpen, onClose, onCreate }: CreateFolderModalProps) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleClose = () => {
    if (isSubmitting) return
    setName('')
    setDescription('')
    setError(null)
    onClose()
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
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
      await onCreate(trimmedName, description.trim())
      setName('')
      setDescription('')
      onClose()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to create album'
      setError(msg)
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title="Create New Album"
      description="Create a virtual album to organize your indexed photos without moving original files."
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
          <label htmlFor="folder-name" className="block text-xs font-medium text-foreground mb-1.5">
            Album Name <span className="text-destructive">*</span>
          </label>
          <Input
            id="folder-name"
            value={name}
            onChange={(e) => {
              setName(e.target.value)
              if (error) setError(null)
            }}
            placeholder="e.g. Summer Roadtrip 2026"
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
          <label htmlFor="folder-desc" className="block text-xs font-medium text-foreground mb-1.5">
            Description (Optional)
          </label>
          <Input
            id="folder-desc"
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
              <FolderPlus className="w-4 h-4" />
            )}
            <span>{isSubmitting ? 'Creating...' : 'Create Album'}</span>
          </Button>
        </div>
      </form>
    </Modal>
  )
}
