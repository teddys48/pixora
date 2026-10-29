import { useEffect, useRef, useCallback, useState } from 'react'

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
 * - Properly manages sentinel DOM element attachment via state
 * - Prevents rapid re-triggers when loading is already in progress
 * - Ignores intersection when `hasMore` is false or `disabled` is true
 * - Re-evaluates viewport visibility on initial sentinel attachment and after loading finishes
 * - Automatically cleans up observer on unmount or sentinel change
 */
export function useInfiniteScroll({
  onLoadMore,
  hasMore,
  isLoading,
  threshold = 0,
  rootMargin = '400px 0px',
  disabled = false,
}: UseInfiniteScrollOptions) {
  const [sentinelNode, setSentinelNode] = useState<HTMLDivElement | null>(null)

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

  // Manage IntersectionObserver attachment to sentinel node
  useEffect(() => {
    if (!sentinelNode || !hasMore || disabled) return

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

    observer.observe(sentinelNode)

    return () => {
      observer.disconnect()
    }
  }, [sentinelNode, hasMore, disabled, triggerLoadMore, rootMargin, threshold])

  // Re-check viewport visibility when loading finishes, hasMore changes, or sentinel attaches
  // In case the sentinel is already in or near the viewport (e.g., initial render or large displays)
  useEffect(() => {
    if (!sentinelNode || !hasMore || isLoading || disabled) return

    const rect = sentinelNode.getBoundingClientRect()
    const marginMatch = rootMargin.match(/^(-?\d+)px/)
    const marginOffset = marginMatch ? parseInt(marginMatch[1], 10) : 400

    if (rect.top <= window.innerHeight + marginOffset && rect.bottom >= -marginOffset) {
      triggerLoadMore()
    }
  }, [sentinelNode, hasMore, isLoading, disabled, rootMargin, triggerLoadMore])

  const sentinelRef = useCallback((node: HTMLDivElement | null) => {
    setSentinelNode(node)
  }, [])

  return { sentinelRef }
}
