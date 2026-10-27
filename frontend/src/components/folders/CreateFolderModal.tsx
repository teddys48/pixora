import React, { useState } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { FolderPlus, Info } from 'lucide-react'

interface CreateFolderModalProps {
  isOpen: boolean
  onClose: () => void
  onCreate: (name: string, description: string) => void
}

export function CreateFolderModal({ isOpen, onClose, onCreate }: CreateFolderModalProps) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    onCreate(name.trim(), description.trim())
    setName('')
    setDescription('')
    onClose()
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Create New Folder"
      description="Organize your photos into curated albums."
      maxWidth="md"
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label htmlFor="folder-name" className="block text-xs font-medium text-foreground mb-1.5">
            Folder Name <span className="text-destructive">*</span>
          </label>
          <Input
            id="folder-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. Summer Roadtrip 2026"
            required
            autoFocus
          />
        </div>

        <div>
          <label htmlFor="folder-desc" className="block text-xs font-medium text-foreground mb-1.5">
            Description (Optional)
          </label>
          <Input
            id="folder-desc"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Brief summary of photos in this folder"
          />
        </div>

        {/* Prototype disclaimer badge */}
        <div className="flex items-start gap-2.5 p-3 rounded-lg bg-muted/60 border border-border/80 text-xs text-muted-foreground">
          <Info className="w-4 h-4 shrink-0 text-muted-foreground mt-0.5" />
          <p>
            <strong>Phase 1 Prototype:</strong> Folders created here are held in local state for UI demonstration. Persistent database storage will be added in <strong>Phase 4: Folder Management</strong>.
          </p>
        </div>

        <div className="flex items-center justify-end gap-2 pt-3 border-t border-border">
          <Button type="button" variant="outline" size="sm" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" size="sm" disabled={!name.trim()} className="gap-1.5">
            <FolderPlus className="w-4 h-4" />
            <span>Create Folder</span>
          </Button>
        </div>
      </form>
    </Modal>
  )
}
