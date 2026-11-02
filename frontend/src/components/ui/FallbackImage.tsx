import React, { useState } from 'react'
import { ImageOff, Camera, Film } from 'lucide-react'
import { cn } from '@/lib/utils'

interface FallbackImageProps extends React.ImgHTMLAttributes<HTMLImageElement> {
  fallbackSrc?: string
  alt: string
  aspectRatio?: 'video' | 'square' | 'portrait' | 'auto'
  imgClassName?: string
  isVideo?: boolean
}

export function FallbackImage({
  src,
  alt,
  className,
  imgClassName,
  aspectRatio = 'auto',
  isVideo = false,
  ...props
}: FallbackImageProps) {
  const [isLoading, setIsLoading] = useState(true)
  const [hasError, setHasError] = useState(false)

  const aspectClass = {
    video: 'aspect-[16/10]',
    square: 'aspect-square',
    portrait: 'aspect-[3/4]',
    auto: '',
  }[aspectRatio]

  if (hasError || !src) {
    return (
      <div
        className={cn(
          'flex flex-col items-center justify-center bg-muted/60 text-muted-foreground p-4 text-center rounded-md select-none',
          aspectClass,
          className
        )}
      >
        {isVideo ? (
          <Film className="w-8 h-8 mb-2 opacity-50 stroke-[1.5]" />
        ) : (
          <ImageOff className="w-8 h-8 mb-2 opacity-50 stroke-[1.5]" />
        )}
        <span className="text-xs font-medium max-w-[150px] truncate">{alt}</span>
        <span className="text-[10px] text-muted-foreground/70 mt-0.5">
          {isVideo ? 'Video preview unavailable' : 'Image unavailable'}
        </span>
      </div>
    )
  }

  // Determine object-fit styling: prefer explicit imgClassName, or extract object-contain from className
  const objectFitClass = imgClassName
    ? imgClassName
    : className?.includes('object-contain')
    ? 'object-contain'
    : 'object-cover'

  return (
    <div className={cn('relative overflow-hidden bg-muted/30 flex items-center justify-center', aspectClass, className)}>
      {isLoading && (
        <div className="absolute inset-0 flex items-center justify-center bg-muted/40 animate-pulse">
          {isVideo ? (
            <Film className="w-6 h-6 text-muted-foreground/30 animate-pulse" />
          ) : (
            <Camera className="w-6 h-6 text-muted-foreground/30 animate-pulse" />
          )}
        </div>
      )}
      <img
        src={src}
        alt={alt}
        loading="lazy"
        onLoad={() => setIsLoading(false)}
        onError={() => {
          setIsLoading(false)
          setHasError(true)
        }}
        className={cn(
          'w-full h-full transition-opacity duration-300',
          isLoading ? 'opacity-0' : 'opacity-100',
          objectFitClass
        )}
        {...props}
      />
    </div>
  )
}
