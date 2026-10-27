import React from 'react'
import { cn } from '@/lib/utils'

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: 'default' | 'secondary' | 'outline' | 'success'
}

export function Badge({ className, variant = 'default', ...props }: BadgeProps) {
  return (
    <div
      className={cn(
        'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium transition-colors',
        {
          'bg-primary text-primary-foreground': variant === 'default',
          'bg-secondary text-secondary-foreground': variant === 'secondary',
          'border border-border text-foreground': variant === 'outline',
          'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20': variant === 'success',
        },
        className
      )}
      {...props}
    />
  )
}
