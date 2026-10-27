import { Menu, Camera, Upload } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { ThemeToggle } from '@/components/layout/ThemeToggle'

interface HeaderProps {
  onOpenMobileMenu: () => void
  onUploadClick: () => void
}

export function Header({ onOpenMobileMenu, onUploadClick }: HeaderProps) {
  return (
    <header className="sticky top-0 z-30 flex h-14 items-center justify-between border-b border-border bg-background/80 px-4 md:px-6 backdrop-blur-md">
      <div className="flex items-center gap-3">
        <button
          onClick={onOpenMobileMenu}
          aria-label="Open navigation menu"
          className="flex md:hidden items-center justify-center h-9 w-9 rounded-md border border-border text-muted-foreground hover:bg-muted hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
        >
          <Menu className="h-4 w-4" />
        </button>

        <div className="flex items-center gap-2 md:hidden">
          <div className="flex items-center justify-center w-7 h-7 rounded-lg bg-primary text-primary-foreground">
            <Camera className="w-4 h-4" />
          </div>
          <span className="font-semibold text-base tracking-tight text-foreground">Pixora</span>
        </div>
      </div>

      <div className="flex items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          onClick={onUploadClick}
          className="flex md:hidden gap-1.5 h-8 text-xs font-medium"
        >
          <Upload className="w-3.5 h-3.5" />
          <span>Upload</span>
        </Button>
        <ThemeToggle className="md:hidden" />
      </div>
    </header>
  )
}
