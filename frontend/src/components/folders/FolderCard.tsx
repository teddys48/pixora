import React from 'react'
import { Folder } from '@/types/folder'
import { FallbackImage } from '@/components/ui/FallbackImage'
import { formatDate } from '@/lib/utils'
import { Folder as FolderIcon, Images } from 'lucide-react'

interface FolderCardProps {
  folder: Folder
  onClick: (folder: Folder) => void
}

export function FolderCard({ folder, onClick }: FolderCardProps) {
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
      className="group flex flex-col rounded-xl overflow-hidden border border-border/70 bg-card hover:border-border transition-all duration-300 hover:shadow-lg cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
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
