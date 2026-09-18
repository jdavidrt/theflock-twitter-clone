import { Link, useNavigate, useParams } from 'react-router-dom';
import { useMutation } from '@tanstack/react-query';
import Avatar from '../components/Avatar.js';
import ComposeBox from '../components/ComposeBox.js';
import TweetCard from '../components/TweetCard.js';
import { useAuth } from '../context/AuthContext.js';
import { createReply, deleteTweet } from '../lib/api.js';
import { useThread } from '../lib/useThread.js';
import './TweetDetail.css';

// D-48 thread page: ancestor chain (deleted ones as a placeholder, D-49), the focused tweet,
// an inline reply composer, and the paginated replies list.
export default function TweetDetail() {
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const { user: currentUser } = useAuth();
  const thread = useThread(id);
  const currentUsername = currentUser?.username ?? '';

  const replyMutation = useMutation({
    mutationFn: (content: string) => createReply(id, content),
    onSuccess: thread.appendReply,
  });

  // Deleting the tweet you're currently viewing leaves nothing sensible to show in its place,
  // so this navigates away instead of trying to keep the thread view in sync.
  const deleteFocusedMutation = useMutation({
    mutationFn: () => deleteTweet(id),
    onSuccess: () => navigate('/'),
  });

  if (thread.isLoading) return <p className="tweet-detail__status">Loading…</p>;
  if (thread.isError || !thread.tweet) {
    return <p className="tweet-detail__status tweet-detail__status--error">Tweet not found.</p>;
  }

  return (
    <div className="tweet-detail">
      {thread.ancestors.map((ancestor) =>
        ancestor.isDeleted ? (
          <p key={ancestor.id} className="tweet-detail__deleted-ancestor">
            This tweet was deleted
          </p>
        ) : (
          <Link key={ancestor.id} to={`/tweet/${ancestor.id}`} className="tweet-detail__ancestor">
            <Avatar
              username={ancestor.author.username}
              displayName={ancestor.author.displayName}
              size={32}
            />
            <span className="tweet-detail__ancestor-name">{ancestor.author.displayName}</span>
            <span className="tweet-detail__ancestor-username">@{ancestor.author.username}</span>
            <span className="tweet-detail__ancestor-content">{ancestor.content}</span>
          </Link>
        ),
      )}

      <TweetCard
        tweet={thread.tweet}
        isOwn={thread.tweet.author.username === currentUsername}
        onLikeToggle={() => thread.tweet && thread.likeMutation.mutate(thread.tweet)}
        onDelete={() => deleteFocusedMutation.mutate()}
      />

      <div className="tweet-detail__composer">
        <ComposeBox onSubmit={(content) => replyMutation.mutateAsync(content)} />
      </div>

      <ul className="tweet-detail__replies">
        {thread.replies.map((reply) => (
          <li key={reply.id}>
            <p className="tweet-detail__replying-to">
              Replying to @{thread.tweet?.author.username}
            </p>
            <TweetCard
              tweet={reply}
              isOwn={reply.author.username === currentUsername}
              onLikeToggle={() => thread.likeMutation.mutate(reply)}
              onDelete={() => thread.deleteReplyMutation.mutate(reply.id)}
            />
          </li>
        ))}
      </ul>

      {thread.hasNextPage && (
        <>
          <div ref={thread.sentinelRef} aria-hidden="true" />
          <button
            type="button"
            className="tweet-detail__load-more"
            onClick={() => thread.fetchNextPage()}
            disabled={thread.isFetchingNextPage}
          >
            {thread.isFetchingNextPage ? 'Loading…' : 'Load more replies'}
          </button>
        </>
      )}
    </div>
  );
}
