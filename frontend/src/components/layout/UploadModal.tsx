import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { UploadCloud, CheckCircle2, ShieldAlert } from 'lucide-react'

interface UploadModalProps {
  isOpen: boolean
  onClose: () => void
}

export function UploadModal({ isOpen, onClose }: UploadModalProps) {
  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Upload Photos"
      description="Add high-resolution photos to your Pixora personal gallery."
      maxWidth="md"
    >
      <div className="space-y-4">
        {/* Mock Drag & Drop Area */}
        <div className="flex flex-col items-center justify-center p-8 border-2 border-dashed border-border/80 rounded-xl bg-muted/20 text-center hover:bg-muted/30 transition-colors">
          <div className="flex items-center justify-center w-12 h-12 rounded-full bg-primary/10 text-primary mb-3">
            <UploadCloud className="w-6 h-6" />
          </div>
          <p className="text-sm font-medium text-foreground mb-1">
            Drag and drop images here, or browse
          </p>
          <p className="text-xs text-muted-foreground max-w-xs">
            Supports JPEG, PNG, WebP up to 25MB each.
          </p>
        </div>

        {/* Phase 1 Notice Box */}
        <div className="flex items-start gap-3 p-3.5 rounded-lg bg-amber-500/10 border border-amber-500/20 text-amber-900 dark:text-amber-200 text-xs">
          <ShieldAlert className="w-4 h-4 shrink-0 text-amber-600 dark:text-amber-400 mt-0.5" />
          <div className="space-y-1">
            <p className="font-semibold text-amber-800 dark:text-amber-300">Phase 1 Preview</p>
            <p className="text-muted-foreground dark:text-muted-foreground/90 leading-relaxed">
              Photo uploads, backend validation, thumbnail pipelines, and EXIF processing will be implemented in <strong>Phase 3: Upload & Photo Management</strong>.
            </p>
          </div>
        </div>

        {/* Footer actions */}
        <div className="flex items-center justify-end gap-2 pt-2 border-t border-border">
          <Button variant="outline" size="sm" onClick={onClose}>
            Cancel
          </Button>
          <Button size="sm" onClick={onClose} className="gap-1.5">
            <CheckCircle2 className="w-4 h-4" />
            <span>Got it</span>
          </Button>
        </div>
      </div>
    </Modal>
  )
}
