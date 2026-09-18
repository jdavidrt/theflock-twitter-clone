import type { InfiniteData } from '@tanstack/react-query';
import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getThread, likeTweet, unlikeTweet, deleteTweet } from './api.js';
import type { Thread, Tweet } from './api.js';
import { useInfiniteScrollSentinel } from './useInfiniteScrollSentinel.js';

type ThreadData = InfiniteData<Thread, string | null>;

// Maps every reply across every cached page, dropping the ones `update` returns null for.
function mapReplies(
  old: ThreadData | undefined,
  update: (tweet: Tweet) => Tweet | null,
): ThreadData | undefined {
  if (!old) return old;
  return {
    ...old,
    pages: old.pages.map((page) => ({
      ...page,
      replies: {
        ...page.replies,
        items: page.replies.items.map(update).filter((t): t is Tweet => t !== null),
      },
    })),
  };
}

// The focused tweet is identical on every page (only `replies` changes page to page), so it
// only needs updating on the first one — that's the only copy the UI reads.
function updateFocusedTweet(
  old: ThreadData | undefined,
  update: (tweet: Tweet) => Tweet,
): ThreadData | undefined {
  if (!old || old.pages.length === 0) return old;
  const [first, ...rest] = old.pages;
  return { ...old, pages: [{ ...first, tweet: update(first.tweet) }, ...rest] };
}

// A thread page (D-48): the ancestor chain and focused tweet from the first page, replies
// flattened across every fetched page, with optimistic like/unlike (on the focused tweet and
// any reply) and reply deletion, mirroring useTweetFeed's pattern for the home/profile feeds.
export function useThread(tweetId: string) {
  const queryClient = useQueryClient();
  const queryKey = ['thread', tweetId];

  const { data, isLoading, isError, fetchNextPage, hasNextPage, isFetchingNextPage } =
    useInfiniteQuery({
      queryKey,
      queryFn: ({ pageParam }) => getThread(tweetId, pageParam),
      initialPageParam: null as string | null,
      getNextPageParam: (lastPage) => lastPage.replies.nextCursor,
    });

  const sentinelRef = useInfiniteScrollSentinel(hasNextPage ?? false, fetchNextPage);

  const likeMutation = useMutation({
    mutationFn: (tweet: Tweet) => (tweet.likedByMe ? unlikeTweet(tweet.id) : likeTweet(tweet.id)),
    onMutate: async (tweet) => {
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<ThreadData>(queryKey);
      const toggle = (t: Tweet): Tweet =>
        t.id === tweet.id
          ? { ...t, likedByMe: !t.likedByMe, likeCount: t.likeCount + (t.likedByMe ? -1 : 1) }
          : t;
      queryClient.setQueryData<ThreadData>(queryKey, (old) =>
        updateFocusedTweet(mapReplies(old, toggle), toggle),
      );
      return { previous };
    },
    onError: (_err, _tweet, context) => {
      if (context?.previous) queryClient.setQueryData(queryKey, context.previous);
    },
  });

  const deleteReplyMutation = useMutation({
    mutationFn: deleteTweet,
    onMutate: async (id: string) => {
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<ThreadData>(queryKey);
      queryClient.setQueryData<ThreadData>(queryKey, (old) =>
        mapReplies(old, (t) => (t.id === id ? null : t)),
      );
      return { previous };
    },
    onError: (_err, _id, context) => {
      if (context?.previous) queryClient.setQueryData(queryKey, context.previous);
    },
  });

  // Used by the reply composer to append a freshly-created reply to the last cached page.
  function appendReply(reply: Tweet) {
    queryClient.setQueryData<ThreadData>(queryKey, (old) => {
      if (!old || old.pages.length === 0) return old;
      const pages = [...old.pages];
      const last = pages.length - 1;
      pages[last] = {
        ...pages[last],
        replies: { ...pages[last].replies, items: [...pages[last].replies.items, reply] },
      };
      return { ...old, pages };
    });
  }

  return {
    ancestors: data?.pages[0]?.ancestors ?? [],
    tweet: data?.pages[0]?.tweet ?? null,
    replies: data?.pages.flatMap((page) => page.replies.items) ?? [],
    isLoading,
    isError,
    hasNextPage: hasNextPage ?? false,
    isFetchingNextPage,
    fetchNextPage,
    sentinelRef,
    likeMutation,
    deleteReplyMutation,
    appendReply,
  };
}
