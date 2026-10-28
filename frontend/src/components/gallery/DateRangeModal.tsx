import { useState, useEffect, useMemo } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { formatDate } from '@/lib/utils'
import {
  ChevronLeft,
  ChevronRight,
  Calendar as CalendarIcon,
  RotateCcw,
  Check,
  AlertCircle,
} from 'lucide-react'
import { cn } from '@/lib/utils'

interface DateRangeModalProps {
  isOpen: boolean
  onClose: () => void
  startDate: string // 'YYYY-MM-DD'
  endDate: string // 'YYYY-MM-DD'
  onApply: (startDate: string, endDate: string) => void
  onClear?: () => void
}

function toDateStr(d: Date): string {
  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function parseDateStr(str: string): Date | null {
  if (!str) return null
  const parts = str.split('-').map(Number)
  if (parts.length < 3 || isNaN(parts[0]) || isNaN(parts[1]) || isNaN(parts[2])) {
    return null
  }
  return new Date(parts[0], parts[1] - 1, parts[2])
}

const MONTH_NAMES = [
  'January',
  'February',
  'March',
  'April',
  'May',
  'June',
  'July',
  'August',
  'September',
  'October',
  'November',
  'December',
]

const DAY_LABELS = ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa']

export function DateRangeModal({
  isOpen,
  onClose,
  startDate,
  endDate,
  onApply,
  onClear,
}: DateRangeModalProps) {
  const [tempStart, setTempStart] = useState(startDate)
  const [tempEnd, setTempEnd] = useState(endDate)
  const [hoverDate, setHoverDate] = useState<string | null>(null)

  // Current calendar view (year and month 0-11)
  const initialDate = parseDateStr(startDate) || parseDateStr(endDate) || new Date()
  const [currentYear, setCurrentYear] = useState(initialDate.getFullYear())
  const [currentMonth, setCurrentMonth] = useState(initialDate.getMonth())

  // Reset internal state whenever modal opens or external props change
  useEffect(() => {
    if (isOpen) {
      setTempStart(startDate)
      setTempEnd(endDate)
      const d = parseDateStr(startDate) || parseDateStr(endDate) || new Date()
      setCurrentYear(d.getFullYear())
      setCurrentMonth(d.getMonth())
      setHoverDate(null)
    }
  }, [isOpen, startDate, endDate])

  const prevMonth = () => {
    if (currentMonth === 0) {
      setCurrentMonth(11)
      setCurrentYear((y) => y - 1)
    } else {
      setCurrentMonth((m) => m - 1)
    }
  }

  const nextMonth = () => {
    if (currentMonth === 11) {
      setCurrentMonth(0)
      setCurrentYear((y) => y + 1)
    } else {
      setCurrentMonth((m) => m + 1)
    }
  }

  // Quick preset ranges
  const presets = useMemo(() => {
    const now = new Date()
    return [
      {
        label: 'Today',
        action: () => {
          const s = toDateStr(now)
          setTempStart(s)
          setTempEnd(s)
          setCurrentYear(now.getFullYear())
          setCurrentMonth(now.getMonth())
        },
      },
      {
        label: 'Last 7 Days',
        action: () => {
          const end = new Date()
          const start = new Date()
          start.setDate(start.getDate() - 6)
          setTempStart(toDateStr(start))
          setTempEnd(toDateStr(end))
          setCurrentYear(end.getFullYear())
          setCurrentMonth(end.getMonth())
        },
      },
      {
        label: 'Last 30 Days',
        action: () => {
          const end = new Date()
          const start = new Date()
          start.setDate(start.getDate() - 29)
          setTempStart(toDateStr(start))
          setTempEnd(toDateStr(end))
          setCurrentYear(end.getFullYear())
          setCurrentMonth(end.getMonth())
        },
      },
      {
        label: 'This Month',
        action: () => {
          const start = new Date(now.getFullYear(), now.getMonth(), 1)
          const end = new Date(now.getFullYear(), now.getMonth() + 1, 0)
          setTempStart(toDateStr(start))
          setTempEnd(toDateStr(end))
          setCurrentYear(now.getFullYear())
          setCurrentMonth(now.getMonth())
        },
      },
      {
        label: 'Last Month',
        action: () => {
          const start = new Date(now.getFullYear(), now.getMonth() - 1, 1)
          const end = new Date(now.getFullYear(), now.getMonth(), 0)
          setTempStart(toDateStr(start))
          setTempEnd(toDateStr(end))
          setCurrentYear(start.getFullYear())
          setCurrentMonth(start.getMonth())
        },
      },
      {
        label: 'This Year',
        action: () => {
          const start = new Date(now.getFullYear(), 0, 1)
          const end = new Date(now.getFullYear(), 11, 31)
          setTempStart(toDateStr(start))
          setTempEnd(toDateStr(end))
          setCurrentYear(now.getFullYear())
          setCurrentMonth(now.getMonth())
        },
      },
    ]
  }, [])

  // Days in current month grid
  const daysInMonth = useMemo(() => {
    const totalDays = new Date(currentYear, currentMonth + 1, 0).getDate()
    const firstDayIndex = new Date(currentYear, currentMonth, 1).getDay()
    const days: { dateStr: string; dayNum: number }[] = []

    for (let i = 1; i <= totalDays; i++) {
      const d = new Date(currentYear, currentMonth, i)
      days.push({
        dateStr: toDateStr(d),
        dayNum: i,
      })
    }
    return { firstDayIndex, days }
  }, [currentYear, currentMonth])

  const handleDayClick = (dateStr: string) => {
    if (!tempStart || (tempStart && tempEnd)) {
      // Starting new selection
      setTempStart(dateStr)
      setTempEnd('')
    } else {
      // Completing selection
      if (dateStr < tempStart) {
        setTempEnd(tempStart)
        setTempStart(dateStr)
      } else {
        setTempEnd(dateStr)
      }
    }
  }

  const isInvalidRange = Boolean(tempStart && tempEnd && tempStart > tempEnd)

  const handleApply = () => {
    if (isInvalidRange) return
    onApply(tempStart, tempEnd)
    onClose()
  }

  const handleClear = () => {
    setTempStart('')
    setTempEnd('')
    onClear?.()
    onApply('', '')
    onClose()
  }

  // Generate year options for dropdown
  const yearOptions = useMemo(() => {
    const years: number[] = []
    const current = new Date().getFullYear()
    for (let y = current + 1; y >= 1990; y--) {
      years.push(y)
    }
    return years
  }, [])

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Filter by Capture Date Range"
      description="Choose a start and end date to find photos captured within that period."
      maxWidth="lg"
    >
      <div className="space-y-5">
        {/* Date Inputs & Summary */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 p-3.5 rounded-lg border border-border/70 bg-muted/30">
          <div>
            <label className="text-[11px] font-medium text-muted-foreground block mb-1">
              Start Date (From)
            </label>
            <input
              type="date"
              value={tempStart}
              onChange={(e) => setTempStart(e.target.value)}
              className="w-full h-8 px-2.5 rounded-md border border-input bg-card text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
            />
          </div>
          <div>
            <label className="text-[11px] font-medium text-muted-foreground block mb-1">
              End Date (To)
            </label>
            <input
              type="date"
              value={tempEnd}
              onChange={(e) => setTempEnd(e.target.value)}
              className="w-full h-8 px-2.5 rounded-md border border-input bg-card text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
            />
          </div>
        </div>

        {isInvalidRange && (
          <div className="flex items-center gap-2 p-2.5 rounded-md border border-destructive/30 bg-destructive/10 text-destructive text-xs">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>Start date cannot be after end date.</span>
          </div>
        )}

        {/* Quick Presets */}
        <div>
          <span className="text-[11px] font-semibold text-muted-foreground uppercase tracking-wider block mb-2">
            Quick Presets
          </span>
          <div className="flex flex-wrap gap-1.5">
            {presets.map((p) => (
              <button
                key={p.label}
                type="button"
                onClick={p.action}
                className="px-2.5 py-1 rounded-md text-xs font-medium border border-border bg-card hover:bg-muted text-foreground transition-colors"
              >
                {p.label}
              </button>
            ))}
          </div>
        </div>

        {/* Interactive Month Calendar */}
        <div className="border border-border rounded-xl p-3 bg-card/60">
          {/* Calendar Header with Month/Year Navigation */}
          <div className="flex items-center justify-between mb-3 px-1">
            <div className="flex items-center gap-1.5">
              <select
                value={currentMonth}
                onChange={(e) => setCurrentMonth(Number(e.target.value))}
                className="h-8 px-2 rounded-md border border-input bg-card text-xs font-semibold text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
              >
                {MONTH_NAMES.map((name, idx) => (
                  <option key={name} value={idx}>
                    {name}
                  </option>
                ))}
              </select>
              <select
                value={currentYear}
                onChange={(e) => setCurrentYear(Number(e.target.value))}
                className="h-8 px-2 rounded-md border border-input bg-card text-xs font-semibold text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
              >
                {yearOptions.map((y) => (
                  <option key={y} value={y}>
                    {y}
                  </option>
                ))}
              </select>
            </div>

            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={prevMonth}
                aria-label="Previous month"
                className="p-1.5 rounded-md border border-border hover:bg-muted text-muted-foreground hover:text-foreground transition-colors"
              >
                <ChevronLeft className="w-4 h-4" />
              </button>
              <button
                type="button"
                onClick={nextMonth}
                aria-label="Next month"
                className="p-1.5 rounded-md border border-border hover:bg-muted text-muted-foreground hover:text-foreground transition-colors"
              >
                <ChevronRight className="w-4 h-4" />
              </button>
            </div>
          </div>

          {/* Days of Week Header */}
          <div className="grid grid-cols-7 gap-1 text-center mb-1">
            {DAY_LABELS.map((day) => (
              <div
                key={day}
                className="text-[11px] font-semibold text-muted-foreground py-1"
              >
                {day}
              </div>
            ))}
          </div>

          {/* Calendar Grid */}
          <div className="grid grid-cols-7 gap-1">
            {/* Blank offset days */}
            {Array.from({ length: daysInMonth.firstDayIndex }).map((_, idx) => (
              <div key={`blank-${idx}`} className="h-8" />
            ))}

            {/* Month Day Cells */}
            {daysInMonth.days.map(({ dateStr, dayNum }) => {
              const isStart = dateStr === tempStart
              const isEnd = dateStr === tempEnd
              const isInSelectedRange =
                Boolean(tempStart && tempEnd && dateStr > tempStart && dateStr < tempEnd)
              const isInHoverRange =
                Boolean(
                  tempStart &&
                    !tempEnd &&
                    hoverDate &&
                    hoverDate > tempStart &&
                    dateStr > tempStart &&
                    dateStr <= hoverDate
                )

              return (
                <button
                  key={dateStr}
                  type="button"
                  onClick={() => handleDayClick(dateStr)}
                  onMouseEnter={() => setHoverDate(dateStr)}
                  onMouseLeave={() => setHoverDate(null)}
                  className={cn(
                    'h-8 text-xs font-medium transition-all relative flex items-center justify-center',
                    isStart && isEnd && 'rounded-lg bg-primary text-primary-foreground font-bold shadow-xs',
                    isStart && !isEnd && 'rounded-l-lg bg-primary text-primary-foreground font-bold shadow-xs',
                    isEnd && !isStart && 'rounded-r-lg bg-primary text-primary-foreground font-bold shadow-xs',
                    isInSelectedRange && 'bg-primary/15 text-primary rounded-none',
                    isInHoverRange && 'bg-primary/10 text-primary rounded-none',
                    !isStart && !isEnd && !isInSelectedRange && !isInHoverRange && 'rounded-lg hover:bg-muted text-foreground'
                  )}
                >
                  {dayNum}
                </button>
              )
            })}
          </div>
        </div>

        {/* Selected Range Description */}
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <CalendarIcon className="w-3.5 h-3.5 text-primary" />
          <span>
            {tempStart && tempEnd ? (
              <>
                Selected range: <strong>{formatDate(tempStart)}</strong> to{' '}
                <strong>{formatDate(tempEnd)}</strong>
              </>
            ) : tempStart ? (
              <>
                Selected from <strong>{formatDate(tempStart)}</strong> (click another day to set end date)
              </>
            ) : tempEnd ? (
              <>
                Selected up to <strong>{formatDate(tempEnd)}</strong>
              </>
            ) : (
              'No date range selected (showing all dates)'
            )}
          </span>
        </div>

        {/* Action Controls */}
        <div className="flex items-center justify-between pt-3 border-t border-border">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={handleClear}
            className="text-xs gap-1.5 text-muted-foreground hover:text-destructive"
          >
            <RotateCcw className="w-3.5 h-3.5" />
            <span>Clear Dates</span>
          </Button>

          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={onClose}
              className="text-xs"
            >
              Cancel
            </Button>
            <Button
              type="button"
              size="sm"
              onClick={handleApply}
              disabled={isInvalidRange}
              className="text-xs gap-1.5"
            >
              <Check className="w-3.5 h-3.5" />
              <span>Apply Range</span>
            </Button>
          </div>
        </div>
      </div>
    </Modal>
  )
}
