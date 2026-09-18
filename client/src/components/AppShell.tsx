import { NavLink, useLocation, useNavigate } from 'react-router-dom';
import type { ReactNode } from 'react';
import { useAuth } from '../context/AuthContext.js';
import { HomeIcon, LogoutIcon, ProfileIcon, SearchIcon } from './icons.js';
import './AppShell.css';

// Mobile-first layout per D-28: bottom tab bar under 640px, an icon-only left rail from
// 640px, a labeled sidebar from 1024px (CSS media queries in AppShell.css do the switching —
// the same three nav items and logout action render at every breakpoint).
function pageTitle(pathname: string): string {
  return pathname === '/' ? 'Home' : 'The Flock';
}

export default function AppShell({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();

  if (!user) return null;

  async function handleLogout() {
    await logout();
    navigate('/login', { replace: true });
  }

  const navItems = [
    { to: '/', label: 'Home', icon: HomeIcon, end: true },
    { to: '/search', label: 'Search', icon: SearchIcon, end: false },
    { to: `/${user.username}`, label: user.username, icon: ProfileIcon, end: false },
  ];

  return (
    <div className="app-shell">
      <header className="app-shell__topbar">
        <span className="app-shell__topbar-title">{pageTitle(location.pathname)}</span>
        <button
          type="button"
          className="app-shell__logout"
          onClick={handleLogout}
          aria-label="Log out"
        >
          <LogoutIcon />
        </button>
      </header>

      <nav className="app-shell__rail" aria-label="Primary">
        {navItems.map(({ to, label, icon: ItemIcon, end }) => (
          <NavLink
            key={to}
            to={to}
            end={end}
            className={({ isActive }) =>
              'app-shell__nav-item' + (isActive ? ' app-shell__nav-item--active' : '')
            }
          >
            <ItemIcon />
            <span className="app-shell__nav-label">{label}</span>
          </NavLink>
        ))}
        <button
          type="button"
          className="app-shell__nav-item app-shell__rail-logout"
          onClick={handleLogout}
        >
          <LogoutIcon />
          <span className="app-shell__nav-label">Log out</span>
        </button>
      </nav>

      <main className="app-shell__main">
        <div className="app-shell__content">{children}</div>
      </main>

      <nav className="app-shell__bottombar" aria-label="Primary">
        {navItems.map(({ to, label, icon: ItemIcon, end }) => (
          <NavLink
            key={to}
            to={to}
            end={end}
            className={({ isActive }) =>
              'app-shell__nav-item' + (isActive ? ' app-shell__nav-item--active' : '')
            }
          >
            <ItemIcon />
            <span className="app-shell__nav-label">{label}</span>
          </NavLink>
        ))}
      </nav>
    </div>
  );
}
