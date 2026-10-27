import React from 'react'
import { cn } from '@/lib/utils'

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'outline' | 'ghost' | 'destructive'
  size?: 'sm' | 'md' | 'lg' | 'icon'
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = 'primary', size = 'md', children, disabled, ...props }, ref) => {
    return (
      <button
        ref={ref}
        disabled={disabled}
        className={cn(
          'inline-flex items-center justify-center font-medium transition-colors select-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 active:scale-[0.98]',
          {
            // Variants
            'bg-primary text-primary-foreground hover:bg-primary/90 shadow-sm': variant === 'primary',
            'bg-secondary text-secondary-foreground hover:bg-secondary/80': variant === 'secondary',
            'border border-border bg-transparent hover:bg-accent hover:text-accent-foreground': variant === 'outline',
            'hover:bg-accent hover:text-accent-foreground': variant === 'ghost',
            'bg-destructive text-destructive-foreground hover:bg-destructive/90': variant === 'destructive',

            // Sizes
            'h-8 px-3 text-xs rounded-md gap-1.5': size === 'sm',
            'h-9 px-4 text-sm rounded-md gap-2': size === 'md',
            'h-11 px-6 text-base rounded-lg gap-2.5': size === 'lg',
            'h-9 w-9 rounded-md p-0': size === 'icon',
          },
          className
        )}
        {...props}
      >
        {children}
      </button>
    )
  }
)

Button.displayName = 'Button'
