import { render } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import App from '../src/App.js';
import { AuthProvider } from '../src/context/AuthContext.js';

// Renders the real App (routes, auth context and all) so integration tests exercise the same
// wiring as the browser, starting from initialEntries instead of BrowserRouter's real URL.
export function renderApp(options: { initialEntries: string[] }) {
  const queryClient = new QueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={options.initialEntries}>
        <AuthProvider>
          <App />
        </AuthProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}
