import { NavLink } from 'react-router-dom'
import { Images, Calendar, FolderOpen, Upload, Camera } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { HealthBadge } from '@/components/layout/HealthBadge'
import { ThemeToggle } from '@/components/layout/ThemeToggle'
import { cn } from '@/lib/utils'

interface SidebarProps {
  onUploadClick: () => void
  className?: string
  onCloseMobile?: () => void
}

export function Sidebar({ onUploadClick, className, onCloseMobile }: SidebarProps) {
  const navItems = [
    { to: '/', label: 'Photos', icon: Images, end: true },
    { to: '/timeline', label: 'Timeline', icon: Calendar, end: false },
    { to: '/folders', label: 'Folders', icon: FolderOpen, end: false },
  ]

  return (
    <aside
      className={cn(
        'flex flex-col h-full bg-card/60 backdrop-blur-md border-r border-border p-4 select-none',
        className
      )}
    >
      {/* Brand Header */}
      <div className="flex items-center gap-3 px-2 py-3 mb-6">
        <div className="flex items-center justify-center w-9 h-9 rounded-xl bg-primary text-primary-foreground shadow-md">
          <Camera className="w-5 h-5 stroke-[2]" />
        </div>
        <div className="flex flex-col">
          <div className="flex items-center gap-1.5">
            <span className="font-semibold text-base tracking-tight text-foreground">Pixora</span>
            <span className="text-[10px] uppercase tracking-wider font-semibold px-1.5 py-0.2 rounded bg-muted text-muted-foreground border border-border/50">
              v0.1
            </span>
          </div>
          <span className="text-xs text-muted-foreground font-normal">Personal Gallery</span>
        </div>
      </div>

      {/* Primary Action Button */}
      <div className="px-2 mb-6">
        <Button
          onClick={() => {
            onUploadClick()
            if (onCloseMobile) onCloseMobile()
          }}
          className="w-full shadow-sm hover:shadow transition-all group gap-2 h-10 font-medium"
        >
          <Upload className="w-4 h-4 transition-transform group-hover:-translate-y-0.5" />
          <span>Upload Photos</span>
        </Button>
      </div>

      {/* Main Navigation */}
      <nav className="flex-1 space-y-1 px-2" aria-label="Main Navigation">
        {navItems.map((item) => {
          const Icon = item.icon
          return (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              onClick={onCloseMobile}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-primary/10 text-primary font-semibold dark:bg-primary/15 dark:text-foreground'
                    : 'text-muted-foreground hover:bg-muted/70 hover:text-foreground'
                )
              }
            >
              <Icon className="w-4 h-4 shrink-0" />
              <span>{item.label}</span>
            </NavLink>
          )
        })}
      </nav>

      {/* Bottom controls & status */}
      <div className="pt-4 border-t border-border/60 space-y-3 px-2">
        <HealthBadge />
        <div className="flex items-center justify-between pt-1">
          <span className="text-xs text-muted-foreground">Appearance</span>
          <ThemeToggle />
        </div>
      </div>
    </aside>
  )
}
