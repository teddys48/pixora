import { useTheme } from '@/context/ThemeContext'
import { Sun, Moon, Laptop } from 'lucide-react'
import { cn } from '@/lib/utils'

export function ThemeToggle({ className }: { className?: string }) {
  const { theme, setTheme } = useTheme()

  return (
    <div
      role="group"
      aria-label="Theme selector"
      className={cn('inline-flex items-center rounded-lg border border-border bg-muted/50 p-0.5', className)}
    >
      <button
        onClick={() => setTheme('light')}
        aria-label="Light mode"
        aria-pressed={theme === 'light'}
        className={cn(
          'flex items-center justify-center rounded-md p-1.5 text-xs font-medium transition-all focus:outline-none focus-visible:ring-1 focus-visible:ring-ring',
          theme === 'light'
            ? 'bg-background text-foreground shadow-sm'
            : 'text-muted-foreground hover:text-foreground'
        )}
      >
        <Sun className="h-3.5 w-3.5" />
      </button>

      <button
        onClick={() => setTheme('dark')}
        aria-label="Dark mode"
        aria-pressed={theme === 'dark'}
        className={cn(
          'flex items-center justify-center rounded-md p-1.5 text-xs font-medium transition-all focus:outline-none focus-visible:ring-1 focus-visible:ring-ring',
          theme === 'dark'
            ? 'bg-background text-foreground shadow-sm'
            : 'text-muted-foreground hover:text-foreground'
        )}
      >
        <Moon className="h-3.5 w-3.5" />
      </button>

      <button
        onClick={() => setTheme('system')}
        aria-label="System theme"
        aria-pressed={theme === 'system'}
        className={cn(
          'flex items-center justify-center rounded-md p-1.5 text-xs font-medium transition-all focus:outline-none focus-visible:ring-1 focus-visible:ring-ring',
          theme === 'system'
            ? 'bg-background text-foreground shadow-sm'
            : 'text-muted-foreground hover:text-foreground'
        )}
      >
        <Laptop className="h-3.5 w-3.5" />
      </button>
    </div>
  )
}
