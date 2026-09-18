import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterAll, afterEach, beforeAll } from 'vitest';
import { server } from './mocks/server.js';

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => server.resetHandlers());
// vite.config.ts doesn't set test.globals, so @testing-library/react's auto-cleanup (which
// needs a global afterEach) never registers itself — unmount explicitly or DOM from one test
// leaks into the next.
afterEach(() => cleanup());
afterAll(() => server.close());
