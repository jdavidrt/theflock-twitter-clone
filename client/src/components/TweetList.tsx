import type { ReactNode, RefObject } from 'react';
import TweetCard from './TweetCard.js';
import type { Tweet } from '../lib/api.js';
import './TweetList.css';

interface TweetListProps {
  tweets: Tweet[];
  isLoading: boolean;
  isError: boolean;
  emptyState: ReactNode;
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  fetchNextPage: () => void;
  sentinelRef: RefObject<HTMLDivElement>;
  currentUsername: string;
  onLikeToggle: (tweet: Tweet) => void;
  onDelete: (id: string) => void;
}

// Loading/error/empty states plus the infinite-scroll list itself (D-19/D-20): the sentinel
// drives auto-loading, the button is the keyboard/assistive-tech-friendly fallback for the
// same fetchNextPage call. Shared by the home timeline and a profile's tweet list.
export default function TweetList({
  tweets,
  isLoading,
  isError,
  emptyState,
  hasNextPage,
  isFetchingNextPage,
  fetchNextPage,
  sentinelRef,
  currentUsername,
  onLikeToggle,
  onDelete,
}: TweetListProps) {
  if (isLoading) return <p className="tweet-list__status">Loading…</p>;
  if (isError) {
    return (
      <p className="tweet-list__status tweet-list__status--error">
        Couldn't load tweets. Try again.
      </p>
    );
  }

  return (
    <>
      {tweets.length === 0 && emptyState}

      <ul className="tweet-list">
        {tweets.map((tweet) => (
          <li key={tweet.id}>
            <TweetCard
              tweet={tweet}
              isOwn={tweet.author.username === currentUsername}
              onLikeToggle={() => onLikeToggle(tweet)}
              onDelete={() => onDelete(tweet.id)}
            />
          </li>
        ))}
      </ul>

      {hasNextPage && (
        <>
          <div ref={sentinelRef} aria-hidden="true" />
          <button
            type="button"
            className="tweet-list__load-more"
            onClick={() => fetchNextPage()}
            disabled={isFetchingNextPage}
          >
            {isFetchingNextPage ? 'Loading…' : 'Load more'}
          </button>
        </>
      )}
    </>
  );
}
