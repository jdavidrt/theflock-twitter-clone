import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
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

function mockAuthenticated() {
  server.use(http.get('/api/auth/me', () => HttpResponse.json(sampleUser)));
}

describe('compose tweet', () => {
  it('posts a tweet and shows it at the top of the timeline', async () => {
    mockAuthenticated();
    server.use(http.get('/api/timeline', () => HttpResponse.json({ items: [], nextCursor: null })));

    let capturedBody: unknown;
    server.use(
      http.post('/api/tweets', async ({ request }) => {
        capturedBody = await request.json();
        return HttpResponse.json(
          {
            id: 't1',
            author: { id: 'u1', username: 'alice', displayName: 'Alice' },
            content: 'Hello, Flock!',
            parentTweetId: null,
            createdAt: '2026-01-02T00:00:00Z',
            likeCount: 0,
            replyCount: 0,
            likedByMe: false,
          },
          { status: 201 },
        );
      }),
    );

    const user = userEvent.setup();
    renderApp({ initialEntries: ['/'] });

    const input = await screen.findByLabelText(/tweet content/i);
    await user.type(input, 'Hello, Flock!');
    expect(screen.getByText('13/280')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /^post$/i }));

    await waitFor(() => expect(capturedBody).toEqual({ content: 'Hello, Flock!' }));
    expect(await screen.findByText('Hello, Flock!')).toBeInTheDocument();
    expect(input).toHaveValue('');
  });

  it('disables the submit button once content passes 280 code points', async () => {
    mockAuthenticated();

    renderApp({ initialEntries: ['/'] });

    const input = await screen.findByLabelText(/tweet content/i);
    const submit = screen.getByRole('button', { name: /^post$/i });
    expect(submit).toBeDisabled();

    fireEvent.change(input, { target: { value: 'a'.repeat(280) } });
    expect(screen.getByText('280/280')).toBeInTheDocument();
    expect(submit).toBeEnabled();

    fireEvent.change(input, { target: { value: 'a'.repeat(281) } });
    expect(screen.getByText('281/280')).toBeInTheDocument();
    expect(submit).toBeDisabled();
  });
});
