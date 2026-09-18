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

describe('login flow', () => {
  it('logs in with valid credentials and lands on the timeline', async () => {
    server.use(http.post('/api/auth/login', () => HttpResponse.json(sampleUser)));
    const user = userEvent.setup();

    renderApp({ initialEntries: ['/login'] });

    await screen.findByRole('heading', { name: /log in/i });

    await user.type(screen.getByLabelText(/email/i), 'alice@example.com');
    await user.type(screen.getByLabelText(/^password$/i), 'Password123!');
    await user.click(screen.getByRole('button', { name: /log in/i }));

    await waitFor(() => expect(screen.getAllByText('alice').length).toBeGreaterThan(0));
    expect(screen.getByPlaceholderText(/what's happening/i)).toBeInTheDocument();
  });

  it('shows an error on invalid credentials', async () => {
    server.use(
      http.post('/api/auth/login', () =>
        HttpResponse.json(
          { error: { code: 'UNAUTHORIZED', message: 'Invalid email or password' } },
          { status: 401 },
        ),
      ),
    );
    const user = userEvent.setup();

    renderApp({ initialEntries: ['/login'] });

    await screen.findByRole('heading', { name: /log in/i });

    await user.type(screen.getByLabelText(/email/i), 'alice@example.com');
    await user.type(screen.getByLabelText(/^password$/i), 'wrong-password');
    await user.click(screen.getByRole('button', { name: /log in/i }));

    await screen.findByText(/invalid email or password/i);
    expect(screen.getByRole('heading', { name: /log in/i })).toBeInTheDocument();
  });
});
