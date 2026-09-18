import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../mocks/server.js';
import { renderApp } from '../test-utils.js';

const sampleUser = {
  id: 'u1',
  username: 'alice',
  email: 'alice@example.com',
  displayName: 'Alice',
  bio: '',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

const rootTweet = {
  id: 't1',
  author: { id: 'u2', username: 'bob', displayName: 'Bob' },
  content: 'The original tweet',
  parentTweetId: null,
  createdAt: '2026-01-01T00:00:00Z',
  likeCount: 0,
  replyCount: 1,
  likedByMe: false,
  isDeleted: false,
};

const existingReply = {
  id: 't2',
  author: { id: 'u3', username: 'carol', displayName: 'Carol' },
  content: 'An existing reply',
  parentTweetId: 't1',
  createdAt: '2026-01-01T00:05:00Z',
  likeCount: 0,
  replyCount: 0,
  likedByMe: false,
  isDeleted: false,
};

function mockAuthenticated() {
  server.use(http.get('/api/auth/me', () => HttpResponse.json(sampleUser)));
}

describe('tweet thread page', () => {
  it('renders the ancestor chain, focused tweet and replies, and appends a posted reply', async () => {
    mockAuthenticated();
    server.use(
      http.get('/api/tweets/t1', () =>
        HttpResponse.json({
          ancestors: [],
          tweet: rootTweet,
          replies: { items: [existingReply], nextCursor: null },
        }),
      ),
    );

    let capturedBody: unknown;
    server.use(
      http.post('/api/tweets/t1/replies', async ({ request }) => {
        capturedBody = await request.json();
        return HttpResponse.json(
          {
            id: 't3',
            author: { id: 'u1', username: 'alice', displayName: 'Alice' },
            content: 'My new reply',
            parentTweetId: 't1',
            createdAt: '2026-01-01T00:10:00Z',
            likeCount: 0,
            replyCount: 0,
            likedByMe: false,
            isDeleted: false,
          },
          { status: 201 },
        );
      }),
    );

    const user = userEvent.setup();
    renderApp({ initialEntries: ['/tweet/t1'] });

    expect(await screen.findByText('The original tweet')).toBeInTheDocument();
    expect(screen.getByText('An existing reply')).toBeInTheDocument();
    expect(screen.getByText('Replying to @bob')).toBeInTheDocument();

    const input = screen.getByLabelText(/tweet content/i);
    await user.type(input, 'My new reply');
    await user.click(screen.getByRole('button', { name: /^post$/i }));

    await waitFor(() => expect(capturedBody).toEqual({ content: 'My new reply' }));
    expect(await screen.findByText('My new reply')).toBeInTheDocument();
  });

  it('shows a deleted-tweet placeholder for a soft-deleted ancestor', async () => {
    mockAuthenticated();
    server.use(
      http.get('/api/tweets/t2', () =>
        HttpResponse.json({
          ancestors: [{ ...rootTweet, isDeleted: true }],
          tweet: existingReply,
          replies: { items: [], nextCursor: null },
        }),
      ),
    );

    renderApp({ initialEntries: ['/tweet/t2'] });

    expect(await screen.findByText('This tweet was deleted')).toBeInTheDocument();
    expect(screen.queryByText('The original tweet')).not.toBeInTheDocument();
  });
});
