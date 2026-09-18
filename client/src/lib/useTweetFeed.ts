import type { InfiniteData, QueryKey } from '@tanstack/react-query';
import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { deleteTweet, likeTweet, unlikeTweet } from './api.js';
import type { Tweet, TweetPage } from './api.js';
import { useInfiniteScrollSentinel } from './useInfiniteScrollSentinel.js';

type TimelineData = InfiniteData<TweetPage, string | null>;

// Maps every tweet in every page of a tweet-list cache, dropping the ones `update` returns
// null for. Shared by the like and delete optimistic updates below.
function mapTweets(
  old: TimelineData | undefined,
  update: (tweet: Tweet) => Tweet | null,
): TimelineData | undefined {
  if (!old) return old;
  return {
    ...old,
    pages: old.pages.map((page) => ({
      ...page,
      items: page.items.map(update).filter((t): t is Tweet => t !== null),
    })),
  };
}

interface UseTweetFeedOptions {
  queryKey: QueryKey;
  fetchPage: (cursor: string | null) => Promise<TweetPage>;
}

// Infinite-scroll tweet list (D-19/D-20) with optimistic like/delete-and-rollback (D-31),
// backing both the home timeline and a profile's tweet list — the only two feeds in the app.
export function useTweetFeed({ queryKey, fetchPage }: UseTweetFeedOptions) {
  const queryClient = useQueryClient();

  const { data, isLoading, isError, fetchNextPage, hasNextPage, isFetchingNextPage } =
    useInfiniteQuery({
      queryKey,
      queryFn: ({ pageParam }) => fetchPage(pageParam),
      initialPageParam: null as string | null,
      getNextPageParam: (lastPage) => lastPage.nextCursor,
    });

  const sentinelRef = useInfiniteScrollSentinel(hasNextPage ?? false, fetchNextPage);

  const likeMutation = useMutation({
    mutationFn: (tweet: Tweet) => (tweet.likedByMe ? unlikeTweet(tweet.id) : likeTweet(tweet.id)),
    onMutate: async (tweet) => {
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<TimelineData>(queryKey);
      queryClient.setQueryData<TimelineData>(queryKey, (old) =>
        mapTweets(old, (t) =>
          t.id === tweet.id
            ? { ...t, likedByMe: !t.likedByMe, likeCount: t.likeCount + (t.likedByMe ? -1 : 1) }
            : t,
        ),
      );
      return { previous };
    },
    onError: (_err, _tweet, context) => {
      if (context?.previous) queryClient.setQueryData(queryKey, context.previous);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteTweet,
    onMutate: async (id: string) => {
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<TimelineData>(queryKey);
      queryClient.setQueryData<TimelineData>(queryKey, (old) =>
        mapTweets(old, (t) => (t.id === id ? null : t)),
      );
      return { previous };
    },
    onError: (_err, _id, context) => {
      if (context?.previous) queryClient.setQueryData(queryKey, context.previous);
    },
  });

  // Used by Home's compose box to prepend a freshly-created tweet to the first cached page.
  function prependTweet(tweet: Tweet) {
    queryClient.setQueryData<TimelineData>(queryKey, (old) => {
      if (!old || old.pages.length === 0) return old;
      const [first, ...rest] = old.pages;
      return { ...old, pages: [{ ...first, items: [tweet, ...first.items] }, ...rest] };
    });
  }

  return {
    tweets: data?.pages.flatMap((page) => page.items) ?? [],
    isLoading,
    isError,
    hasNextPage: hasNextPage ?? false,
    isFetchingNextPage,
    fetchNextPage,
    sentinelRef,
    likeMutation,
    deleteMutation,
    prependTweet,
  };
}
