import { useMutation } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import ComposeBox from '../components/ComposeBox.js';
import TweetList from '../components/TweetList.js';
import { useAuth } from '../context/AuthContext.js';
import { createTweet, getTimeline } from '../lib/api.js';
import { useTweetFeed } from '../lib/useTweetFeed.js';
import './Home.css';

const TIMELINE_KEY = ['timeline'];

export default function Home() {
  const { user } = useAuth();
  const feed = useTweetFeed({ queryKey: TIMELINE_KEY, fetchPage: getTimeline });

  const createMutation = useMutation({
    mutationFn: createTweet,
    onSuccess: feed.prependTweet,
  });

  if (!user) return null;

  return (
    <div className="home">
      <ComposeBox onSubmit={(content) => createMutation.mutateAsync(content)} />

      <TweetList
        tweets={feed.tweets}
        isLoading={feed.isLoading}
        isError={feed.isError}
        emptyState={
          // D-18
          <div className="home__empty">
            <p>Your timeline is empty — follow people to see their tweets</p>
            <Link to="/search" className="home__empty-link">
              Find people to follow
            </Link>
          </div>
        }
        hasNextPage={feed.hasNextPage}
        isFetchingNextPage={feed.isFetchingNextPage}
        fetchNextPage={feed.fetchNextPage}
        sentinelRef={feed.sentinelRef}
        currentUsername={user.username}
        onLikeToggle={(tweet) => feed.likeMutation.mutate(tweet)}
        onDelete={(id) => feed.deleteMutation.mutate(id)}
      />
    </div>
  );
}
