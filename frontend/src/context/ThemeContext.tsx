import React, { createContext, useContext, useEffect, useState } from 'react'

type Theme = 'light' | 'dark' | 'system'

interface ThemeContextType {
  theme: Theme
  resolvedTheme: 'light' | 'dark'
  setTheme: (theme: Theme) => void
}

const ThemeContext = createContext<ThemeContextType | undefined>(undefined)

const THEME_KEY = 'pixora-theme-preference'

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(() => {
    const saved = localStorage.getItem(THEME_KEY)
    if (saved === 'light' || saved === 'dark' || saved === 'system') {
      return saved
    }
    return 'system'
  })

  const [resolvedTheme, setResolvedTheme] = useState<'light' | 'dark'>(() => {
    if (theme === 'system') {
      return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
    }
    return theme
  })

  useEffect(() => {
    const root = document.documentElement
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')

    const applyTheme = () => {
      let resolved: 'light' | 'dark' = 'light'
      if (theme === 'system') {
        resolved = mediaQuery.matches ? 'dark' : 'light'
      } else {
        resolved = theme
      }

      setResolvedTheme(resolved)

      if (resolved === 'dark') {
        root.classList.add('dark')
      } else {
        root.classList.remove('dark')
      }
    }

    applyTheme()

    const listener = () => {
      if (theme === 'system') {
        applyTheme()
      }
    }

    mediaQuery.addEventListener('change', listener)
    return () => mediaQuery.removeEventListener('change', listener)
  }, [theme])

  const setTheme = (newTheme: Theme) => {
    setThemeState(newTheme)
    localStorage.setItem(THEME_KEY, newTheme)
  }

  return (
    <ThemeContext.Provider value={{ theme, resolvedTheme, setTheme }}>
      {children}
    </ThemeContext.Provider>
  )
}

export function useTheme() {
  const context = useContext(ThemeContext)
  if (!context) {
    throw new Error('useTheme must be used within a ThemeProvider')
  }
  return context
}
