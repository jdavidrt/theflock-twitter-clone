import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import App from './App.js';
import { AuthProvider } from './context/AuthContext.js';
import { ApiError } from './lib/api.js';
import './index.css';

// A 4xx (not found, validation, forbidden, ...) is never transient, so retrying it only
// delays the error UI (e.g. a 404 profile stayed on "Loading profile…" for ~7s under the
// default retry(3) with backoff before this was found and fixed).
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (failureCount, error) =>
        !(error instanceof ApiError && error.status >= 400 && error.status < 500) &&
        failureCount < 3,
    },
  },
});

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuthProvider>
          <App />
        </AuthProvider>
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>,
);
