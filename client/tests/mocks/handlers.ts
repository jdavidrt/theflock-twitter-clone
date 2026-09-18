import { http, HttpResponse } from 'msw';

// Default handlers: logged-out by construction (GET /api/auth/me → 401). Individual tests
// override with server.use(...) for the scenario under test.
export const handlers = [
  http.get('/api/auth/me', () =>
    HttpResponse.json(
      { error: { code: 'UNAUTHORIZED', message: 'Authentication required' } },
      { status: 401 },
    ),
  ),
  // Home fetches the timeline as soon as it mounts; default to an empty page so tests that
  // authenticate but don't care about timeline content aren't forced to mock it too.
  http.get('/api/timeline', () => HttpResponse.json({ items: [], nextCursor: null })),
  // A profile page always fetches the user's tweets alongside the profile itself; same
  // rationale as the timeline default above.
  http.get('/api/users/:username/tweets', () => HttpResponse.json({ items: [], nextCursor: null })),
];
