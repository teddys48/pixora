import React from 'react'
import { PhysicalFolder } from '@/types/folder'
import { FallbackImage } from '@/components/ui/FallbackImage'
import { Folder as FolderIcon, Images, FolderTree, AlertCircle } from 'lucide-react'

interface PhysicalFolderCardProps {
  folder: PhysicalFolder
  onClick: (folder: PhysicalFolder) => void
}

export function PhysicalFolderCard({ folder, onClick }: PhysicalFolderCardProps) {
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
      aria-label={`Open folder ${folder.name}, contains ${folder.photoCount} photos`}
      className="group relative flex flex-col rounded-xl overflow-hidden border border-border/70 bg-card hover:border-border transition-all duration-300 hover:shadow-lg cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      {/* Cover Image Container */}
      <div className="relative aspect-[16/10] w-full overflow-hidden bg-muted/30">
        {folder.coverThumbnailUrl ? (
          <FallbackImage
            src={folder.coverThumbnailUrl}
            alt={folder.name}
            className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500 ease-out"
          />
        ) : (
          <div className="flex flex-col items-center justify-center w-full h-full bg-muted/50 text-muted-foreground group-hover:bg-muted/70 transition-colors">
            <FolderIcon className="w-12 h-12 opacity-40 group-hover:opacity-60 transition-opacity" />
          </div>
        )}

        {/* Counter Badge */}
        <div className="absolute top-2.5 right-2.5 flex items-center gap-1.5">
          {folder.subfolderCount > 0 && (
            <span
              title={`${folder.subfolderCount} subfolders`}
              className="flex items-center gap-1 text-[11px] font-medium px-2 py-0.5 rounded-md bg-black/60 text-white backdrop-blur-sm"
            >
              <FolderTree className="w-3 h-3" />
              {folder.subfolderCount}
            </span>
          )}
          <span
            title={`${folder.photoCount} photos`}
            className="flex items-center gap-1 text-[11px] font-medium px-2 py-0.5 rounded-md bg-black/60 text-white backdrop-blur-sm"
          >
            <Images className="w-3 h-3" />
            {folder.photoCount}
          </span>
        </div>

        {/* Unindexed Files Indicator Badge */}
        {folder.hasUnindexedFiles && (
          <div className="absolute top-2.5 left-2.5">
            <span
              title="Folder contains unindexed media files on disk"
              className="flex items-center gap-1 text-[11px] font-medium px-2 py-0.5 rounded-md bg-amber-500/90 text-white backdrop-blur-sm shadow-sm"
            >
              <AlertCircle className="w-3 h-3" />
              <span>Needs scan</span>
            </span>
          </div>
        )}
      </div>

      {/* Footer Info */}
      <div className="p-3.5 flex flex-col justify-between flex-1 bg-card">
        <div className="flex items-center gap-2">
          <FolderIcon className="w-4 h-4 text-primary shrink-0 opacity-80" />
          <h3
            title={folder.name}
            className="font-semibold text-sm text-foreground truncate group-hover:text-primary transition-colors"
          >
            {folder.name}
          </h3>
        </div>

        <div className="mt-2 flex items-center justify-between text-[11px] text-muted-foreground">
          <span className="truncate max-w-[180px]" title={folder.path}>
            {folder.path}
          </span>
          <span className="shrink-0 font-mono text-[10px] bg-muted/60 px-1.5 py-0.5 rounded border border-border/40">
            filesystem
          </span>
        </div>
      </div>
    </div>
  )
}
