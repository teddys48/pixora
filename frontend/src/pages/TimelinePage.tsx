import { useMemo } from 'react'
import { Photo } from '@/types/photo'
import { MOCK_PHOTOS } from '@/data/mock/photos'
import { PhotoCard } from '@/components/gallery/PhotoCard'
import { formatYearMonth } from '@/lib/utils'
import { Calendar, Clock } from 'lucide-react'

interface TimelinePageProps {
  onSelectPhoto: (photo: Photo, photosList: Photo[]) => void
}

interface TimelineGroup {
  periodKey: string // e.g. "2026-10"
  label: string // e.g. "October 2026"
  photos: Photo[]
}

export function TimelinePage({ onSelectPhoto }: TimelinePageProps) {
  // NOTE (Phase 1): This grouping uses mock data timestamps in-memory.
  // In Phase 5 (Timeline & Photo Viewer), this will be backed by SQLite EXIF capture
  // timestamps, database indexing, and paginated timeline queries.
  const groupedTimeline = useMemo<TimelineGroup[]>(() => {
    // Sort all photos chronologically descending first
    const sorted = [...MOCK_PHOTOS].sort(
      (a, b) => new Date(b.date).getTime() - new Date(a.date).getTime()
    )

    const map = new Map<string, TimelineGroup>()

    for (const photo of sorted) {
      const d = new Date(photo.date)
      const periodKey = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
      const label = formatYearMonth(photo.date)

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
  }, [])

  const allTimelinePhotos = useMemo(() => {
    return groupedTimeline.flatMap((g) => g.photos)
  }, [groupedTimeline])

  return (
    <div className="space-y-10">
      {/* Page Header */}
      <div>
        <h1 className="text-2xl font-bold tracking-tight text-foreground flex items-center gap-2">
          <span>Timeline</span>
          <span className="text-xs font-normal text-muted-foreground bg-muted px-2 py-0.5 rounded-full border border-border/60">
            {allTimelinePhotos.length} moments
          </span>
        </h1>
        <p className="text-sm text-muted-foreground mt-1">
          Chronological story of your photography journey, grouped by capture date.
        </p>
      </div>

      {/* Timeline Groups */}
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
    </div>
  )
}
