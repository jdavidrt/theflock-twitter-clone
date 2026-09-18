import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import Avatar from './Avatar.js';
import { LikeIcon, ReplyIcon } from './icons.js';
import type { Tweet } from '../lib/api.js';
import { formatAbsoluteTime, formatRelativeTime } from '../lib/time.js';
import './TweetCard.css';

// D-14: @username tokens are linkified client-side to /:username (regex only, no backend
// mention model). Matches the server's username charset (validation.ts USERNAME_REGEXP).
const MENTION_REGEX = /@([a-zA-Z0-9_]{1,20})/g;

function renderContent(content: string): ReactNode[] {
  const parts: ReactNode[] = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null;
  MENTION_REGEX.lastIndex = 0;
  while ((match = MENTION_REGEX.exec(content))) {
    if (match.index > lastIndex) parts.push(content.slice(lastIndex, match.index));
    parts.push(
      <Link key={match.index} to={`/${match[1]}`} className="tweet-card__mention">
        @{match[1]}
      </Link>,
    );
    lastIndex = match.index + match[0].length;
  }
  if (lastIndex < content.length) parts.push(content.slice(lastIndex));
  return parts;
}

interface TweetCardProps {
  tweet: Tweet;
  isOwn: boolean;
  onLikeToggle: () => void;
  onDelete: () => void;
}

export default function TweetCard({ tweet, isOwn, onLikeToggle, onDelete }: TweetCardProps) {
  function handleDeleteClick() {
    if (window.confirm('Delete this tweet? This cannot be undone.')) {
      onDelete();
    }
  }

  return (
    <article className="tweet-card">
      <Avatar username={tweet.author.username} displayName={tweet.author.displayName} />
      <div className="tweet-card__body">
        <header className="tweet-card__header">
          <Link to={`/${tweet.author.username}`} className="tweet-card__author">
            <span className="tweet-card__display-name">{tweet.author.displayName}</span>
            <span className="tweet-card__username">@{tweet.author.username}</span>
          </Link>
          <span aria-hidden="true">·</span>
          <time
            className="tweet-card__time"
            dateTime={tweet.createdAt}
            title={formatAbsoluteTime(tweet.createdAt)}
          >
            {formatRelativeTime(tweet.createdAt)}
          </time>
        </header>

        <p className="tweet-card__content">{renderContent(tweet.content)}</p>

        <div className="tweet-card__actions">
          <Link to={`/tweet/${tweet.id}`} className="tweet-card__action" aria-label="Replies">
            <ReplyIcon />
            <span>{tweet.replyCount}</span>
          </Link>
          <button
            type="button"
            className={
              'tweet-card__action tweet-card__action--like' +
              (tweet.likedByMe ? ' tweet-card__action--liked' : '')
            }
            onClick={onLikeToggle}
            aria-pressed={tweet.likedByMe}
            aria-label={tweet.likedByMe ? 'Unlike' : 'Like'}
          >
            <LikeIcon filled={tweet.likedByMe} />
            <span>{tweet.likeCount}</span>
          </button>
          {isOwn && (
            <button
              type="button"
              className="tweet-card__action tweet-card__action--delete"
              onClick={handleDeleteClick}
              aria-label="Delete tweet"
            >
              Delete
            </button>
          )}
        </div>
      </div>
    </article>
  );
}
