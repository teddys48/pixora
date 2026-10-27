import React, { useEffect, useRef } from 'react'
import { X } from 'lucide-react'
import { cn } from '@/lib/utils'

interface ModalProps {
  isOpen: boolean
  onClose: () => void
  title?: string
  description?: string
  children: React.ReactNode
  maxWidth?: 'sm' | 'md' | 'lg' | 'xl' | 'full'
  className?: string
}

export function Modal({
  isOpen,
  onClose,
  title,
  description,
  children,
  maxWidth = 'md',
  className,
}: ModalProps) {
  const overlayRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!isOpen) return

    // Lock body scrolling
    const originalStyle = window.getComputedStyle(document.body).overflow
    document.body.style.overflow = 'hidden'

    // Escape key listener
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      }
    }

    window.addEventListener('keydown', handleKeyDown)

    return () => {
      document.body.style.overflow = originalStyle
      window.removeEventListener('keydown', handleKeyDown)
    }
  }, [isOpen, onClose])

  if (!isOpen) return null

  const maxWidthClass = {
    sm: 'max-w-sm',
    md: 'max-w-md',
    lg: 'max-w-lg',
    xl: 'max-w-2xl',
    full: 'max-w-5xl',
  }[maxWidth]

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby={title ? 'modal-title' : undefined}
      aria-describedby={description ? 'modal-description' : undefined}
      className="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-6"
    >
      {/* Backdrop */}
      <div
        ref={overlayRef}
        onClick={onClose}
        className="fixed inset-0 bg-black/60 backdrop-blur-sm transition-opacity"
      />

      {/* Modal Dialog */}
      <div
        className={cn(
          'relative z-10 w-full overflow-hidden rounded-xl border border-border bg-card p-6 shadow-2xl transition-all',
          maxWidthClass,
          className
        )}
      >
        <button
          onClick={onClose}
          aria-label="Close dialog"
          className="absolute right-4 top-4 rounded-md p-1 text-muted-foreground hover:bg-muted hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
        >
          <X className="h-4 w-4" />
        </button>

        {title && (
          <div className="mb-4">
            <h2 id="modal-title" className="text-lg font-semibold tracking-tight text-foreground">
              {title}
            </h2>
            {description && (
              <p id="modal-description" className="text-sm text-muted-foreground mt-1">
                {description}
              </p>
            )}
          </div>
        )}

        {children}
      </div>
    </div>
  )
}
