import React, { useState, useRef } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import {
  UploadCloud,
  CheckCircle2,
  AlertCircle,
  X,
  FileImage,
  Loader2,
} from 'lucide-react'
import { uploadPhotos } from '@/lib/api'
import { formatBytes } from '@/lib/utils'

interface UploadModalProps {
  isOpen: boolean
  onClose: () => void
  onUploadSuccess?: () => void
}

const MAX_FILE_SIZE = 25 * 1024 * 1024 // 25MB
const ALLOWED_EXTENSIONS = ['.jpg', '.jpeg', '.png', '.webp', '.gif', '.tif', '.tiff']
const ALLOWED_MIME_TYPES = [
  'image/jpeg',
  'image/png',
  'image/webp',
  'image/gif',
  'image/tiff',
]

export function UploadModal({ isOpen, onClose, onUploadSuccess }: UploadModalProps) {
  const [selectedFiles, setSelectedFiles] = useState<File[]>([])
  const [isDragging, setIsDragging] = useState(false)
  const [isUploading, setIsUploading] = useState(false)
  const [uploadProgress, setUploadProgress] = useState(0)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [fileErrors, setFileErrors] = useState<{ filename: string; error: string }[]>([])
  const [successCount, setSuccessCount] = useState<number | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const resetState = () => {
    setSelectedFiles([])
    setIsDragging(false)
    setIsUploading(false)
    setUploadProgress(0)
    setErrorMessage(null)
    setFileErrors([])
    setSuccessCount(null)
    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
  }

  const handleClose = () => {
    if (isUploading) return
    resetState()
    onClose()
  }

  const validateAndAddFiles = (incoming: FileList | File[]) => {
    setErrorMessage(null)
    setSuccessCount(null)
    const valid: File[] = []
    const errors: { filename: string; error: string }[] = []

    Array.from(incoming).forEach((file) => {
      const ext = '.' + file.name.split('.').pop()?.toLowerCase()
      const isExtValid = ALLOWED_EXTENSIONS.includes(ext)
      const isMimeValid = ALLOWED_MIME_TYPES.includes(file.type) || file.type === ''

      if (!isExtValid && !isMimeValid) {
        errors.push({
          filename: file.name,
          error: 'Unsupported format. Allowed: JPEG, PNG, WebP, GIF, TIFF.',
        })
        return
      }

      if (file.size > MAX_FILE_SIZE) {
        errors.push({
          filename: file.name,
          error: `File exceeds 25MB limit (${formatBytes(file.size)}).`,
        })
        return
      }

      // Check for duplicates in current selection
      if (selectedFiles.some((f) => f.name === file.name && f.size === file.size)) {
        return
      }

      valid.push(file)
    })

    if (errors.length > 0) {
      setFileErrors(errors)
    }

    if (selectedFiles.length + valid.length > 20) {
      setErrorMessage('Maximum 20 files can be uploaded per batch.')
      const remainingSlots = Math.max(0, 20 - selectedFiles.length)
      setSelectedFiles((prev) => [...prev, ...valid.slice(0, remainingSlots)])
    } else {
      setSelectedFiles((prev) => [...prev, ...valid])
    }
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      validateAndAddFiles(e.target.files)
    }
  }

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault()
    setIsDragging(true)
  }

  const handleDragLeave = () => {
    setIsDragging(false)
  }

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault()
    setIsDragging(false)
    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      validateAndAddFiles(e.dataTransfer.files)
    }
  }

  const removeFile = (index: number) => {
    setSelectedFiles((prev) => prev.filter((_, i) => i !== index))
  }

  const handleUploadSubmit = async () => {
    if (selectedFiles.length === 0 || isUploading) return

    setIsUploading(true)
    setUploadProgress(0)
    setErrorMessage(null)
    setFileErrors([])

    try {
      const result = await uploadPhotos(selectedFiles, (percent) => {
        setUploadProgress(percent)
      })

      setSuccessCount(result.photos.length)
      if (result.errors && result.errors.length > 0) {
        setFileErrors(result.errors)
      }

      if (onUploadSuccess && result.photos.length > 0) {
        onUploadSuccess()
      }

      // Clear uploaded files after successful upload
      setSelectedFiles([])
      if (fileInputRef.current) {
        fileInputRef.current.value = ''
      }
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Upload failed'
      setErrorMessage(message)
    } finally {
      setIsUploading(false)
    }
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title="Upload Photos"
      description="Add high-resolution photos to your Pixora personal gallery."
      maxWidth="md"
    >
      <div className="space-y-4">
        {/* Hidden File Input */}
        <input
          ref={fileInputRef}
          type="file"
          multiple
          accept=".jpg,.jpeg,.png,.webp,.gif,.tif,.tiff,image/jpeg,image/png,image/webp,image/gif,image/tiff"
          className="hidden"
          onChange={handleFileChange}
          disabled={isUploading}
        />

        {/* Drag & Drop Area */}
        <div
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onDrop={handleDrop}
          onClick={() => !isUploading && fileInputRef.current?.click()}
          className={`flex flex-col items-center justify-center p-8 border-2 border-dashed rounded-xl text-center cursor-pointer transition-colors ${
            isDragging
              ? 'border-primary bg-primary/10'
              : 'border-border/80 bg-muted/20 hover:bg-muted/30'
          } ${isUploading ? 'pointer-events-none opacity-60' : ''}`}
        >
          <div className="flex items-center justify-center w-12 h-12 rounded-full bg-primary/10 text-primary mb-3">
            <UploadCloud className="w-6 h-6" />
          </div>
          <p className="text-sm font-medium text-foreground mb-1">
            Drag and drop images here, or browse
          </p>
          <p className="text-xs text-muted-foreground max-w-xs">
            Supports JPEG, PNG, WebP, GIF, TIFF up to 25MB each (max 20 per batch).
          </p>
        </div>

        {/* Top Error Alert */}
        {errorMessage && (
          <div className="flex items-start gap-2.5 p-3 rounded-lg bg-destructive/10 border border-destructive/20 text-destructive text-xs">
            <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
            <p className="leading-relaxed">{errorMessage}</p>
          </div>
        )}

        {/* Success Alert */}
        {successCount !== null && successCount > 0 && (
          <div className="flex items-start gap-2.5 p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-800 dark:text-emerald-300 text-xs">
            <CheckCircle2 className="w-4 h-4 shrink-0 mt-0.5 text-emerald-600 dark:text-emerald-400" />
            <p className="leading-relaxed font-medium">
              Successfully uploaded {successCount} {successCount === 1 ? 'photo' : 'photos'} to your gallery.
            </p>
          </div>
        )}

        {/* Individual File Errors List */}
        {fileErrors.length > 0 && (
          <div className="space-y-1.5 p-3 rounded-lg bg-destructive/10 border border-destructive/20 text-xs">
            <p className="font-semibold text-destructive mb-1">Validation issues:</p>
            {fileErrors.map((fe, idx) => (
              <div key={idx} className="text-muted-foreground flex items-baseline gap-1.5">
                <span className="font-mono text-destructive truncate max-w-[150px]">{fe.filename}:</span>
                <span>{fe.error}</span>
              </div>
            ))}
          </div>
        )}

        {/* Selected Files List */}
        {selectedFiles.length > 0 && (
          <div className="space-y-2">
            <div className="flex items-center justify-between text-xs text-muted-foreground px-1">
              <span>{selectedFiles.length} {selectedFiles.length === 1 ? 'file' : 'files'} selected</span>
              <span>
                Total: {formatBytes(selectedFiles.reduce((acc, f) => acc + f.size, 0))}
              </span>
            </div>

            <div className="max-h-40 overflow-y-auto space-y-1.5 pr-1">
              {selectedFiles.map((file, idx) => (
                <div
                  key={`${file.name}-${idx}`}
                  className="flex items-center justify-between p-2 rounded-lg bg-card border border-border text-xs"
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <FileImage className="w-4 h-4 text-muted-foreground shrink-0" />
                    <span className="font-medium text-foreground truncate max-w-[220px]">
                      {file.name}
                    </span>
                    <span className="text-[11px] text-muted-foreground font-mono">
                      {formatBytes(file.size)}
                    </span>
                  </div>
                  {!isUploading && (
                    <button
                      type="button"
                      onClick={() => removeFile(idx)}
                      className="p-1 text-muted-foreground hover:text-foreground rounded transition-colors"
                      title="Remove file"
                    >
                      <X className="w-3.5 h-3.5" />
                    </button>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Progress Bar */}
        {isUploading && (
          <div className="space-y-1.5 pt-2">
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span className="flex items-center gap-1.5">
                <Loader2 className="w-3.5 h-3.5 animate-spin text-primary" />
                <span>Uploading photos...</span>
              </span>
              <span className="font-mono font-medium">{uploadProgress}%</span>
            </div>
            <div className="w-full h-2 rounded-full bg-muted overflow-hidden">
              <div
                className="h-full bg-primary transition-all duration-200"
                style={{ width: `${uploadProgress}%` }}
              />
            </div>
          </div>
        )}

        {/* Footer actions */}
        <div className="flex items-center justify-end gap-2 pt-3 border-t border-border">
          <Button
            variant="outline"
            size="sm"
            onClick={handleClose}
            disabled={isUploading}
          >
            {successCount !== null && successCount > 0 && selectedFiles.length === 0 ? 'Close' : 'Cancel'}
          </Button>

          {selectedFiles.length > 0 && (
            <Button
              size="sm"
              onClick={handleUploadSubmit}
              disabled={isUploading}
              className="gap-1.5 min-w-[90px]"
            >
              {isUploading ? (
                <>
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                  <span>Uploading</span>
                </>
              ) : (
                <>
                  <UploadCloud className="w-4 h-4" />
                  <span>Upload {selectedFiles.length} {selectedFiles.length === 1 ? 'file' : 'files'}</span>
                </>
              )}
            </Button>
          )}
        </div>
      </div>
    </Modal>
  )
}
