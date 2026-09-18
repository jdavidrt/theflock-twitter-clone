import { useState } from 'react';
import type { FormEvent } from 'react';
import { ApiError } from '../lib/api.js';
import { countCodePoints, TWEET_CONTENT_MAX_LENGTH } from '../lib/validation.js';
import './ComposeBox.css';

interface ComposeBoxProps {
  onSubmit: (content: string) => Promise<unknown>;
}

// Live code-point counter (D-13); submit is disabled when empty or over the limit. Content is
// trimmed before both counting and submitting, matching the server's rule.
export default function ComposeBox({ onSubmit }: ComposeBoxProps) {
  const [content, setContent] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const length = countCodePoints(content.trim());
  const overLimit = length > TWEET_CONTENT_MAX_LENGTH;
  const disabled = submitting || length === 0 || overLimit;

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (disabled) return;
    setSubmitting(true);
    setError(null);
    try {
      await onSubmit(content.trim());
      setContent('');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Something went wrong. Please try again.');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className="compose" onSubmit={handleSubmit}>
      {error && (
        <p className="compose__error" role="alert">
          {error}
        </p>
      )}
      <textarea
        className="compose__input"
        value={content}
        onChange={(e) => setContent(e.target.value)}
        placeholder="What's happening?"
        rows={3}
        aria-label="Tweet content"
      />
      <div className="compose__footer">
        <span className={'compose__counter' + (overLimit ? ' compose__counter--over' : '')}>
          {length}/{TWEET_CONTENT_MAX_LENGTH}
        </span>
        <button type="submit" className="compose__submit" disabled={disabled}>
          {submitting ? 'Posting…' : 'Post'}
        </button>
      </div>
    </form>
  );
}
