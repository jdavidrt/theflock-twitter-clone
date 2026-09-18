import { useEffect, useRef } from 'react';

// D-20: fires fetchNextPage when the sentinel enters the viewport, for any infinite list (the
// visible "Load more" button next to the sentinel is the keyboard/assistive-tech fallback).
export function useInfiniteScrollSentinel(hasNextPage: boolean, fetchNextPage: () => void) {
  const sentinelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const node = sentinelRef.current;
    if (!node || !hasNextPage) return;
    const observer = new IntersectionObserver((entries) => {
      if (entries[0].isIntersecting) fetchNextPage();
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, [hasNextPage, fetchNextPage]);

  return sentinelRef;
}
