import type { InfiniteData } from '@tanstack/react-query';
import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Link, useParams } from 'react-router-dom';
import Avatar from '../components/Avatar.js';
import { useAuth } from '../context/AuthContext.js';
import { follow, getFollowers, getFollowing, unfollow } from '../lib/api.js';
import type { FollowListItem, FollowListPage } from '../lib/api.js';
import { useInfiniteScrollSentinel } from '../lib/useInfiniteScrollSentinel.js';
import './FollowList.css';

type FollowListData = InfiniteData<FollowListPage, string | null>;

interface FollowListProps {
  mode: 'followers' | 'following';
}

// D-24: followers/following, cursor-paginated at 20, each row with a follow toggle (hidden for
// self). Shares D-20's IntersectionObserver-plus-button pattern via useInfiniteScrollSentinel.
export default function FollowList({ mode }: FollowListProps) {
  const { username = '' } = useParams();
  const { user: currentUser } = useAuth();
  const queryClient = useQueryClient();
  const queryKey = ['followList', mode, username];
  const fetchPage = mode === 'followers' ? getFollowers : getFollowing;

  const { data, isLoading, isError, fetchNextPage, hasNextPage, isFetchingNextPage } =
    useInfiniteQuery({
      queryKey,
      queryFn: ({ pageParam }) => fetchPage(username, pageParam),
      initialPageParam: null as string | null,
      getNextPageParam: (lastPage) => lastPage.nextCursor,
    });

  const sentinelRef = useInfiniteScrollSentinel(hasNextPage ?? false, fetchNextPage);

  const toggleMutation = useMutation({
    mutationFn: (item: FollowListItem) =>
      item.isFollowedByMe ? unfollow(item.username) : follow(item.username),
    onMutate: async (item) => {
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<FollowListData>(queryKey);
      queryClient.setQueryData<FollowListData>(queryKey, (old) =>
        old
          ? {
              ...old,
              pages: old.pages.map((page) => ({
                ...page,
                items: page.items.map((it) =>
                  it.id === item.id ? { ...it, isFollowedByMe: !it.isFollowedByMe } : it,
                ),
              })),
            }
          : old,
      );
      return { previous };
    },
    onError: (_err, _item, context) => {
      if (context?.previous) queryClient.setQueryData(queryKey, context.previous);
    },
  });

  const items = data?.pages.flatMap((page) => page.items) ?? [];
  const title = mode === 'followers' ? 'Followers' : 'Following';

  return (
    <div className="follow-list">
      <h1 className="follow-list__title">{title}</h1>

      {isLoading && <p className="follow-list__status">Loading…</p>}
      {isError && (
        <p className="follow-list__status follow-list__status--error">
          Couldn't load this list. Try again.
        </p>
      )}

      {!isLoading && !isError && items.length === 0 && (
        <p className="follow-list__status">
          {mode === 'followers' ? 'No followers yet.' : 'Not following anyone yet.'}
        </p>
      )}

      <ul className="follow-list__items">
        {items.map((item) => (
          <li key={item.id} className="follow-list__item">
            <Link to={`/${item.username}`} className="follow-list__link">
              <Avatar username={item.username} displayName={item.displayName} />
              <span className="follow-list__names">
                <span className="follow-list__display-name">{item.displayName}</span>
                <span className="follow-list__username">@{item.username}</span>
                {item.bio && <span className="follow-list__bio">{item.bio}</span>}
              </span>
            </Link>
            {currentUser?.username !== item.username && (
              <button
                type="button"
                className={
                  'follow-list__follow-btn' +
                  (item.isFollowedByMe ? ' follow-list__follow-btn--following' : '')
                }
                onClick={() => toggleMutation.mutate(item)}
              >
                {item.isFollowedByMe ? 'Following' : 'Follow'}
              </button>
            )}
          </li>
        ))}
      </ul>

      {hasNextPage && (
        <>
          <div ref={sentinelRef} aria-hidden="true" />
          <button
            type="button"
            className="follow-list__load-more"
            onClick={() => fetchNextPage()}
            disabled={isFetchingNextPage}
          >
            {isFetchingNextPage ? 'Loading…' : 'Load more'}
          </button>
        </>
      )}
    </div>
  );
}
