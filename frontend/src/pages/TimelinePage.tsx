import { useState, useEffect, useMemo, useCallback } from 'react'
import { Photo } from '@/types/photo'
import { PhotoCard } from '@/components/gallery/PhotoCard'
import { fetchTimeline } from '@/lib/api'
import { LoadingState } from '@/components/gallery/LoadingState'
import {
  Calendar,
  Clock,
  Images,
  AlertCircle,
  RefreshCw,
  HelpCircle,
  CalendarDays,
} from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { NavLink } from 'react-router-dom'

interface TimelinePageProps {
  onSelectPhoto: (photo: Photo, photosList: Photo[]) => void
}

interface DayGroup {
  dayKey: string // "2026-10-24"
  label: string // "Saturday, October 24, 2026"
  yearMonthKey: string // "2026-10"
  yearMonthLabel: string // "October 2026"
  photos: Photo[]
}

interface MonthSection {
  yearMonthKey: string
  yearMonthLabel: string
  dayGroups: DayGroup[]
  totalPhotos: number
}

function formatDayLabel(isoString: string): string {
  try {
    const d = new Date(isoString)
    if (isNaN(d.getTime())) return isoString
    return d.toLocaleDateString(undefined, {
      weekday: 'long',
      year: 'numeric',
      month: 'long',
      day: 'numeric',
    })
  } catch {
    return isoString
  }
}

function formatMonthLabel(isoString: string): string {
  try {
    const d = new Date(isoString)
    if (isNaN(d.getTime())) return isoString
    return d.toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'long',
    })
  } catch {
    return isoString
  }
}

export function TimelinePage({ onSelectPhoto }: TimelinePageProps) {
  const [timelinePhotos, setTimelinePhotos] = useState<Photo[]>([])
  const [totalPhotos, setTotalPhotos] = useState(0)
  const [hasMore, setHasMore] = useState(false)
  const [nextCursor, setNextCursor] = useState<string | undefined>(undefined)
  const [isLoading, setIsLoading] = useState(true)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const loadInitialTimeline = useCallback(async () => {
    setIsLoading(true)
    setError(null)
    try {
      const data = await fetchTimeline({ limit: 50, offset: 0 })
      setTimelinePhotos(data.photos)
      setTotalPhotos(data.total)
      setHasMore(data.hasMore)
      setNextCursor(data.nextCursor)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load timeline photos'
      setError(msg)
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    loadInitialTimeline()
  }, [loadInitialTimeline])

  const handleLoadMore = async () => {
    if (isLoadingMore || !hasMore) return
    setIsLoadingMore(true)

    try {
      const data = await fetchTimeline({
        limit: 50,
        offset: timelinePhotos.length,
        cursor: nextCursor,
      })

      setTimelinePhotos((prev) => [...prev, ...data.photos])
      setTotalPhotos(data.total)
      setHasMore(data.hasMore)
      setNextCursor(data.nextCursor)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load more photos'
      setError(msg)
    } finally {
      setIsLoadingMore(false)
    }
  };

  // Group photos into Month Sections containing Day Groups, plus a separate Undated Group
  const { monthSections, undatedPhotos } = useMemo(() => {
    const dated: Photo[] = []
    const undated: Photo[] = []

    for (const p of timelinePhotos) {
      if (p.takenAt && !p.isUndated) {
        dated.push(p)
      } else {
        undated.push(p)
      }
    }

    // Map day groups for dated photos
    const dayMap = new Map<string, DayGroup>()

    for (const p of dated) {
      const d = new Date(p.takenAt!)
      const dayKey = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(
        d.getDate()
      ).padStart(2, '0')}`
      const yearMonthKey = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`

      if (!dayMap.has(dayKey)) {
        dayMap.set(dayKey, {
          dayKey,
          label: formatDayLabel(p.takenAt!),
          yearMonthKey,
          yearMonthLabel: formatMonthLabel(p.takenAt!),
          photos: [],
        })
      }
      dayMap.get(dayKey)!.photos.push(p)
    }

    // Group day groups under Month sections
    const monthMap = new Map<string, MonthSection>()

    for (const dayGroup of dayMap.values()) {
      if (!monthMap.has(dayGroup.yearMonthKey)) {
        monthMap.set(dayGroup.yearMonthKey, {
          yearMonthKey: dayGroup.yearMonthKey,
          yearMonthLabel: dayGroup.yearMonthLabel,
          dayGroups: [],
          totalPhotos: 0,
        })
      }
      const section = monthMap.get(dayGroup.yearMonthKey)!
      section.dayGroups.push(dayGroup)
      section.totalPhotos += dayGroup.photos.length
    }

    return {
      monthSections: Array.from(monthMap.values()),
      undatedPhotos: undated,
    }
  }, [timelinePhotos])

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

  if (error && timelinePhotos.length === 0) {
    return (
      <div className="space-y-6">
        <div className="flex items-center justify-between p-4 rounded-xl border border-destructive/20 bg-destructive/10 text-destructive text-xs">
          <div className="flex items-center gap-2">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{error}</span>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={loadInitialTimeline}
            className="h-7 text-xs ml-3"
          >
            Retry
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-8">
      {/* Page Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground flex items-center gap-2.5">
            <CalendarDays className="w-6 h-6 text-primary" />
            <span>Timeline</span>
            <span className="text-xs font-normal text-muted-foreground bg-muted px-2.5 py-0.5 rounded-full border border-border/60">
              {totalPhotos} {totalPhotos === 1 ? 'photo' : 'photos'}
            </span>
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            Chronological journey through your photos, grouped by capture date with verified EXIF timestamps.
          </p>
        </div>
      </div>

      {/* Empty State when no photos exist at all */}
      {timelinePhotos.length === 0 ? (
        <div className="flex flex-col items-center justify-center p-14 text-center border border-dashed border-border rounded-2xl bg-card/40 my-8">
          <div className="flex items-center justify-center w-14 h-14 rounded-full bg-primary/10 text-primary mb-4">
            <Images className="w-7 h-7 stroke-[1.5]" />
          </div>
          <h3 className="text-lg font-semibold text-foreground mb-1">No photos indexed yet</h3>
          <p className="text-sm text-muted-foreground max-w-sm mb-5 leading-relaxed">
            Upload photos or trigger a media scan from the Photos tab to start building your chronological timeline.
          </p>
          <NavLink to="/">
            <Button size="md">Go to Photos</Button>
          </NavLink>
        </div>
      ) : (
        <div className="space-y-12">
          {/* Dated Chronological Sections */}
          {monthSections.map((monthSec) => (
            <div key={monthSec.yearMonthKey} className="space-y-8">
              {/* Month Context Header */}
              <div className="sticky top-14 z-10 bg-background/95 backdrop-blur-sm py-2.5 border-b border-border/60 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <div className="w-2.5 h-2.5 rounded-full bg-primary" />
                  <h2 className="text-lg font-bold text-foreground tracking-tight">
                    {monthSec.yearMonthLabel}
                  </h2>
                </div>
                <span className="text-xs text-muted-foreground font-medium">
                  {monthSec.totalPhotos} {monthSec.totalPhotos === 1 ? 'moment' : 'moments'}
                </span>
              </div>

              {/* Day Groups inside this Month */}
              <div className="space-y-8 pl-1 sm:pl-3">
                {monthSec.dayGroups.map((dayGroup) => (
                  <section key={dayGroup.dayKey} className="space-y-3.5">
                    {/* Day Group Heading */}
                    <div className="flex items-center justify-between pb-1.5 border-b border-border/30">
                      <div className="flex items-center gap-2 text-foreground font-semibold text-sm sm:text-base">
                        <Calendar className="w-4 h-4 text-primary shrink-0" />
                        <h3>{dayGroup.label}</h3>
                      </div>
                      <span className="text-xs text-muted-foreground flex items-center gap-1 font-mono">
                        <Clock className="w-3 h-3 opacity-70" />
                        {dayGroup.photos.length} {dayGroup.photos.length === 1 ? 'photo' : 'photos'}
                      </span>
                    </div>

                    {/* Day Photos Grid */}
                    <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-3 sm:gap-4 md:gap-5">
                      {dayGroup.photos.map((photo) => (
                        <PhotoCard
                          key={photo.id}
                          photo={photo}
                          onSelect={(p) => onSelectPhoto(p, timelinePhotos)}
                          viewMode="grid"
                        />
                      ))}
                    </div>
                  </section>
                ))}
              </div>
            </div>
          ))}

          {/* Undated Photos Fallback Section */}
          {undatedPhotos.length > 0 && (
            <div className="space-y-6 pt-6 border-t-2 border-dashed border-border/70">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 p-4 rounded-xl border border-border/70 bg-muted/20">
                <div className="flex items-start gap-3">
                  <HelpCircle className="w-5 h-5 text-muted-foreground shrink-0 mt-0.5" />
                  <div>
                    <h3 className="text-base font-semibold text-foreground flex items-center gap-2">
                      <span>Undated Photos</span>
                      <span className="text-xs font-normal text-muted-foreground bg-muted px-2 py-0.5 rounded-full border border-border/60">
                        {undatedPhotos.length} {undatedPhotos.length === 1 ? 'photo' : 'photos'}
                      </span>
                    </h3>
                    <p className="text-xs text-muted-foreground mt-0.5 leading-relaxed">
                      These photos lack valid EXIF capture date metadata. To preserve chronological accuracy,
                      dates are not fabricated; photos are ordered by file modification time.
                    </p>
                  </div>
                </div>
              </div>

              {/* Undated Photos Grid */}
              <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-3 sm:gap-4 md:gap-5">
                {undatedPhotos.map((photo) => (
                  <PhotoCard
                    key={photo.id}
                    photo={photo}
                    onSelect={(p) => onSelectPhoto(p, timelinePhotos)}
                    viewMode="grid"
                  />
                ))}
              </div>
            </div>
          )}

          {/* Incremental Loading / Load More */}
          {hasMore && (
            <div className="flex flex-col items-center justify-center pt-8 pb-4 gap-2">
              <Button
                variant="outline"
                size="md"
                onClick={handleLoadMore}
                disabled={isLoadingMore}
                className="gap-2 px-6"
              >
                {isLoadingMore ? (
                  <RefreshCw className="w-4 h-4 animate-spin text-primary" />
                ) : (
                  <CalendarDays className="w-4 h-4 text-primary" />
                )}
                <span>
                  {isLoadingMore
                    ? 'Loading photos...'
                    : `Load More Moments (${timelinePhotos.length} of ${totalPhotos})`}
                </span>
              </Button>
              <p className="text-xs text-muted-foreground font-mono">
                Showing {timelinePhotos.length} of {totalPhotos} photos
              </p>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
