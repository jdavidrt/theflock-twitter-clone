import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../mocks/server.js';
import { renderApp } from '../test-utils.js';

const alice = {
  id: 'u1',
  username: 'alice',
  email: 'alice@example.com',
  displayName: 'Alice',
  bio: '',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

function bobProfile(overrides: { isFollowedByMe: boolean; followerCount: number }) {
  return {
    id: 'u2',
    username: 'bob',
    displayName: 'Bob',
    bio: 'Just Bob',
    createdAt: '2026-01-01T00:00:00Z',
    tweetCount: 3,
    followingCount: 2,
    ...overrides,
  };
}

function mockAuthenticated() {
  server.use(http.get('/api/auth/me', () => HttpResponse.json(alice)));
}

describe('follow flow', () => {
  it('follows a user and updates the button and follower count', async () => {
    mockAuthenticated();
    server.use(
      http.get('/api/users/bob', () =>
        HttpResponse.json(bobProfile({ isFollowedByMe: false, followerCount: 5 })),
      ),
    );

    let capturedMethod: string | undefined;
    server.use(
      http.post('/api/users/bob/follow', ({ request }) => {
        capturedMethod = request.method;
        return HttpResponse.json(bobProfile({ isFollowedByMe: true, followerCount: 6 }));
      }),
    );

    const user = userEvent.setup();
    renderApp({ initialEntries: ['/bob'] });

    await screen.findByRole('heading', { name: 'Bob' });
    expect(screen.getByText('5')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Follow' }));

    await waitFor(() => expect(capturedMethod).toBe('POST'));
    expect(await screen.findByRole('button', { name: 'Following' })).toBeInTheDocument();
    expect(screen.getByText('6')).toBeInTheDocument();
  });

  it('unfollows a user and updates the button and follower count', async () => {
    mockAuthenticated();
    server.use(
      http.get('/api/users/bob', () =>
        HttpResponse.json(bobProfile({ isFollowedByMe: true, followerCount: 6 })),
      ),
    );

    let capturedMethod: string | undefined;
    server.use(
      http.delete('/api/users/bob/follow', ({ request }) => {
        capturedMethod = request.method;
        return HttpResponse.json(bobProfile({ isFollowedByMe: false, followerCount: 5 }));
      }),
    );

    const user = userEvent.setup();
    renderApp({ initialEntries: ['/bob'] });

    await screen.findByRole('button', { name: 'Following' });
    expect(screen.getByText('6')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Following' }));

    await waitFor(() => expect(capturedMethod).toBe('DELETE'));
    expect(await screen.findByRole('button', { name: 'Follow' })).toBeInTheDocument();
    expect(screen.getByText('5')).toBeInTheDocument();
  });
});
