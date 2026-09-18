import type { ReactNode } from 'react';
import './AuthLayout.css';

interface AuthLayoutProps {
  title: string;
  children: ReactNode;
  footer: ReactNode;
}

// Shared chrome for /login and /register: both are a centered card with a title, a form
// slot, and a footer link to the other page.
export default function AuthLayout({ title, children, footer }: AuthLayoutProps) {
  return (
    <div className="auth-layout">
      <div className="auth-layout__card">
        <h1 className="auth-layout__title">{title}</h1>
        {children}
        <p className="auth-layout__footer">{footer}</p>
      </div>
    </div>
  );
}
