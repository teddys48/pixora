import { useMemo } from 'react'
import { Photo } from '@/types/photo'
import { PhotoCard } from '@/components/gallery/PhotoCard'
import { formatYearMonth } from '@/lib/utils'
import { usePhotos } from '@/context/PhotosContext'
import { LoadingState } from '@/components/gallery/LoadingState'
import { Calendar, Clock, Info, Images, AlertCircle } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { NavLink } from 'react-router-dom'

interface TimelinePageProps {
  onSelectPhoto: (photo: Photo, photosList: Photo[]) => void
}

interface TimelineGroup {
  periodKey: string // e.g. "2026-10"
  label: string // e.g. "October 2026"
  photos: Photo[]
}

export function TimelinePage({ onSelectPhoto }: TimelinePageProps) {
  const { photos, isLoading, error, loadPhotos } = usePhotos()

  // Filter only photos with verified EXIF capture dates.
  // Note: We deliberately do not invent fake capture dates for photos that lack EXIF metadata.
  const photosWithCaptureDate = useMemo(() => {
    return photos.filter((p): p is Photo & { takenAt: string } => Boolean(p.takenAt))
  }, [photos])

  const omittedCount = photos.length - photosWithCaptureDate.length

  const groupedTimeline = useMemo<TimelineGroup[]>(() => {
    // Sort all photos with capture dates chronologically descending
    const sorted = [...photosWithCaptureDate].sort(
      (a, b) => new Date(b.takenAt).getTime() - new Date(a.takenAt).getTime()
    )

    const map = new Map<string, TimelineGroup>()

    for (const photo of sorted) {
      const d = new Date(photo.takenAt)
      const periodKey = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
      const label = formatYearMonth(photo.takenAt)

      if (!map.has(periodKey)) {
        map.set(periodKey, {
          periodKey,
          label,
          photos: [],
        })
      }

      map.get(periodKey)!.photos.push(photo)
    }

    return Array.from(map.values())
  }, [photosWithCaptureDate])

  const allTimelinePhotos = useMemo(() => {
    return groupedTimeline.flatMap((g) => g.photos)
  }, [groupedTimeline])

  if (isLoading) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">Timeline</h1>
          <p className="text-sm text-muted-foreground mt-1">Loading chronological photo story...</p>
        </div>
        <LoadingState viewMode="grid" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div className="flex items-center justify-between p-4 rounded-xl border border-destructive/20 bg-destructive/10 text-destructive text-xs">
          <div className="flex items-center gap-2">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{error}</span>
          </div>
          <Button variant="outline" size="sm" onClick={() => loadPhotos(true)} className="h-7 text-xs ml-3">
            Retry
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-8">
      {/* Page Header */}
      <div>
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
          <h1 className="text-2xl font-bold tracking-tight text-foreground flex items-center gap-2">
            <span>Timeline</span>
            <span className="text-xs font-normal text-muted-foreground bg-muted px-2 py-0.5 rounded-full border border-border/60">
              {allTimelinePhotos.length} {allTimelinePhotos.length === 1 ? 'moment' : 'moments'}
            </span>
          </h1>
          <span className="text-xs font-medium text-primary bg-primary/10 px-2.5 py-1 rounded-full border border-primary/20 self-start sm:self-auto">
            Phase 5 Preview
          </span>
        </div>
        <p className="text-sm text-muted-foreground mt-1">
          Chronological story of your photography journey, grouped by verified EXIF capture date.
        </p>
      </div>

      {/* Phase 5 Notice Banner */}
      <div className="flex items-start gap-3 p-3.5 rounded-xl border border-border/70 bg-card/60 text-xs text-muted-foreground">
        <Info className="w-4 h-4 text-primary shrink-0 mt-0.5" />
        <div className="space-y-1">
          <p className="font-medium text-foreground">Timeline Feature Status</p>
          <p className="leading-relaxed">
            Full chronological querying, database-level date grouping, and infinite scroll are planned for{' '}
            <strong>Phase 5</strong>. Currently displaying real indexed photos that contain valid EXIF capture
            dates (<code>DateTimeOriginal</code>). Photos without EXIF capture dates are not forced into the
            timeline to prevent inaccurate chronology.
          </p>
          {omittedCount > 0 && (
            <p className="text-primary font-medium pt-0.5">
              Note: {omittedCount} {omittedCount === 1 ? 'photo' : 'photos'} without EXIF capture timestamps{' '}
              {omittedCount === 1 ? 'is' : 'are'} omitted from this view. You can view all photos in the{' '}
              <NavLink to="/" className="underline hover:text-foreground">
                Photos tab
              </NavLink>
              .
            </p>
          )}
        </div>
      </div>

      {/* Empty State when no photos exist at all */}
      {photos.length === 0 ? (
        <div className="flex flex-col items-center justify-center p-14 text-center border border-dashed border-border rounded-2xl bg-card/40 my-8">
          <div className="flex items-center justify-center w-14 h-14 rounded-full bg-primary/10 text-primary mb-4">
            <Images className="w-7 h-7 stroke-[1.5]" />
          </div>
          <h3 className="text-lg font-semibold text-foreground mb-1">No photos indexed yet</h3>
          <p className="text-sm text-muted-foreground max-w-sm mb-5 leading-relaxed">
            Upload photos or trigger a media scan from the Photos tab to populate your gallery and timeline.
          </p>
          <NavLink to="/">
            <Button size="md">Go to Photos</Button>
          </NavLink>
        </div>
      ) : photosWithCaptureDate.length === 0 ? (
        /* Empty State when photos exist but none have EXIF capture timestamps */
        <div className="flex flex-col items-center justify-center p-14 text-center border border-dashed border-border rounded-2xl bg-card/40 my-8">
          <div className="flex items-center justify-center w-14 h-14 rounded-full bg-muted text-muted-foreground mb-4">
            <Calendar className="w-7 h-7 stroke-[1.5]" />
          </div>
          <h3 className="text-lg font-semibold text-foreground mb-1">No EXIF capture dates found</h3>
          <p className="text-sm text-muted-foreground max-w-md mb-5 leading-relaxed">
            None of the {photos.length} currently indexed photos contain EXIF <code>DateTimeOriginal</code> metadata.
            To preserve chronological accuracy, capture dates are not invented. In Phase 5, customizable fallback
            date views will be supported.
          </p>
          <NavLink to="/">
            <Button variant="outline" size="md">
              Browse all {photos.length} photos
            </Button>
          </NavLink>
        </div>
      ) : (
        /* Timeline Groups */
        <div className="space-y-12 relative before:absolute before:inset-0 before:left-3 md:before:left-4 before:w-0.5 before:bg-border/60">
          {groupedTimeline.map((group) => (
            <section key={group.periodKey} className="relative pl-8 md:pl-10 space-y-4">
              {/* Timeline Marker Node */}
              <div className="absolute left-1.5 md:left-2.5 top-1 -translate-x-1/2 flex items-center justify-center w-5 h-5 rounded-full bg-background border-2 border-primary text-primary shadow-xs">
                <div className="w-1.5 h-1.5 rounded-full bg-primary" />
              </div>

              {/* Date Group Header */}
              <div className="flex items-center justify-between pb-2 border-b border-border/40">
                <div className="flex items-center gap-2">
                  <Calendar className="w-4 h-4 text-primary" />
                  <h2 className="text-lg font-semibold text-foreground tracking-tight">
                    {group.label}
                  </h2>
                </div>
                <span className="text-xs text-muted-foreground flex items-center gap-1">
                  <Clock className="w-3 h-3" />
                  {group.photos.length} {group.photos.length === 1 ? 'photo' : 'photos'}
                </span>
              </div>

              {/* Photos Grid for this period */}
              <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 md:gap-6 pt-1">
                {group.photos.map((photo) => (
                  <PhotoCard
                    key={photo.id}
                    photo={photo}
                    onSelect={(p) => onSelectPhoto(p, allTimelinePhotos)}
                    viewMode="grid"
                  />
                ))}
              </div>
            </section>
          ))}
        </div>
      )}
    </div>
  )
}
