import { useEffect, useRef, useCallback } from 'react'

export interface UseInfiniteScrollOptions {
  onLoadMore: () => void | Promise<void>
  hasMore: boolean
  isLoading: boolean
  threshold?: number
  rootMargin?: string
  disabled?: boolean
}

/**
 * useInfiniteScroll attaches an IntersectionObserver to a sentinel DOM element
 * and invokes `onLoadMore` when approaching the end of the loaded content.
 *
 * Built-in safeguards:
 * - Prevents rapid re-triggers when loading is already in progress
 * - Ignores intersection when `hasMore` is false
 * - Re-evaluates viewport visibility on sentinel attachment
 * - Automatically cleans up observer on unmount
 */
export function useInfiniteScroll({
  onLoadMore,
  hasMore,
  isLoading,
  threshold = 0,
  rootMargin = '400px 0px',
  disabled = false,
}: UseInfiniteScrollOptions) {
  const sentinelNodeRef = useRef<HTMLDivElement | null>(null)
  const onLoadMoreRef = useRef(onLoadMore)
  onLoadMoreRef.current = onLoadMore

  const isLoadingRef = useRef(isLoading)
  isLoadingRef.current = isLoading

  const hasMoreRef = useRef(hasMore)
  hasMoreRef.current = hasMore

  const disabledRef = useRef(disabled)
  disabledRef.current = disabled

  const isTriggeringRef = useRef(false)

  const triggerLoadMore = useCallback(async () => {
    if (
      !hasMoreRef.current ||
      isLoadingRef.current ||
      disabledRef.current ||
      isTriggeringRef.current
    ) {
      return
    }

    try {
      isTriggeringRef.current = true
      await onLoadMoreRef.current()
    } finally {
      // Debounce trigger unlock to prevent rapid duplicate intersection dispatches
      setTimeout(() => {
        isTriggeringRef.current = false
      }, 150)
    }
  }, [])

  useEffect(() => {
    const target = sentinelNodeRef.current
    if (!target) return

    const observer = new IntersectionObserver(
      (entries) => {
        const first = entries[0]
        if (first?.isIntersecting) {
          triggerLoadMore()
        }
      },
      {
        rootMargin,
        threshold,
      }
    )

    observer.observe(target)

    return () => {
      observer.disconnect()
    }
  }, [triggerLoadMore, rootMargin, threshold])

  const sentinelRef = useCallback(
    (node: HTMLDivElement | null) => {
      sentinelNodeRef.current = node
      if (node && hasMoreRef.current && !isLoadingRef.current && !disabledRef.current) {
        const rect = node.getBoundingClientRect()
        if (rect.top <= window.innerHeight + 400) {
          triggerLoadMore()
        }
      }
    },
    [triggerLoadMore]
  )

  return { sentinelRef }
}
