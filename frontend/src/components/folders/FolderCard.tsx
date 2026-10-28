import React from 'react'
import { Folder } from '@/types/folder'
import { FallbackImage } from '@/components/ui/FallbackImage'
import { formatDate } from '@/lib/utils'
import { Folder as FolderIcon, Images, MoreVertical, Edit2, Trash2 } from 'lucide-react'

interface FolderCardProps {
  folder: Folder
  onClick: (folder: Folder) => void
  onRename?: (folder: Folder) => void
  onDelete?: (folder: Folder) => void
}

export function FolderCard({ folder, onClick, onRename, onDelete }: FolderCardProps) {
  const [showMenu, setShowMenu] = React.useState(false)
  const menuRef = React.useRef<HTMLDivElement>(null)

  React.useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setShowMenu(false)
      }
    }
    if (showMenu) {
      document.addEventListener('mousedown', handleClickOutside)
    }
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [showMenu])

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      onClick(folder)
    }
  }

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() => onClick(folder)}
      onKeyDown={handleKeyDown}
      className="group relative flex flex-col rounded-xl overflow-hidden border border-border/70 bg-card hover:border-border transition-all duration-300 hover:shadow-lg cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      {/* Cover Image Container */}
      <div className="relative aspect-[16/10] w-full overflow-hidden bg-muted/30">
        {folder.coverUrl ? (
          <FallbackImage
            src={folder.coverUrl}
            alt={folder.name}
            className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500 ease-out"
          />
        ) : (
          <div className="flex items-center justify-center w-full h-full bg-muted/60 text-muted-foreground">
            <FolderIcon className="w-12 h-12 opacity-40" />
          </div>
        )}

        {/* Counter Badge */}
        <div className="absolute top-2.5 right-2.5">
          <span className="flex items-center gap-1.5 text-xs font-medium px-2 py-1 rounded-md bg-black/60 text-white backdrop-blur-sm">
            <Images className="w-3 h-3" />
            {folder.photoCount}
          </span>
        </div>

        {/* Actions Button / Dropdown */}
        {(onRename || onDelete) && (
          <div
            ref={menuRef}
            className="absolute top-2.5 left-2.5"
            onClick={(e) => e.stopPropagation()}
          >
            <button
              type="button"
              onClick={() => setShowMenu((prev) => !prev)}
              aria-label="Album actions"
              className="p-1.5 rounded-md bg-black/60 hover:bg-black/80 text-white backdrop-blur-sm transition-colors opacity-90 group-hover:opacity-100"
            >
              <MoreVertical className="w-3.5 h-3.5" />
            </button>

            {showMenu && (
              <div className="absolute left-0 mt-1 w-32 rounded-lg bg-popover/95 backdrop-blur border border-border shadow-md py-1 z-20 text-xs animate-in fade-in zoom-in-95 duration-100">
                {onRename && (
                  <button
                    type="button"
                    onClick={() => {
                      setShowMenu(false)
                      onRename(folder)
                    }}
                    className="flex items-center gap-2 w-full px-3 py-1.5 text-left text-foreground hover:bg-muted/80 transition-colors"
                  >
                    <Edit2 className="w-3.5 h-3.5 text-muted-foreground" />
                    <span>Rename</span>
                  </button>
                )}
                {onDelete && (
                  <button
                    type="button"
                    onClick={() => {
                      setShowMenu(false)
                      onDelete(folder)
                    }}
                    className="flex items-center gap-2 w-full px-3 py-1.5 text-left text-destructive hover:bg-destructive/10 transition-colors"
                  >
                    <Trash2 className="w-3.5 h-3.5 text-destructive" />
                    <span>Delete</span>
                  </button>
                )}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Folder Details */}
      <div className="p-4 flex flex-col justify-between flex-1">
        <div>
          <h3 className="font-semibold text-base text-foreground group-hover:text-primary transition-colors flex items-center gap-2">
            <FolderIcon className="w-4 h-4 text-muted-foreground shrink-0 group-hover:text-primary transition-colors" />
            <span className="truncate">{folder.name}</span>
          </h3>
          {folder.description && (
            <p className="text-xs text-muted-foreground line-clamp-2 mt-1 leading-relaxed">
              {folder.description}
            </p>
          )}
        </div>

        <div className="mt-4 pt-3 border-t border-border/50 flex items-center justify-between text-[11px] text-muted-foreground">
          <span>Updated {formatDate(folder.updatedAt)}</span>
          <span className="font-medium text-foreground text-xs opacity-0 group-hover:opacity-100 transition-opacity">
            Browse &rarr;
          </span>
        </div>
      </div>
    </div>
  )
}
