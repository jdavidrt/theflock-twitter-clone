import { useState } from 'react';
import type { FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import AuthLayout from '../components/AuthLayout.js';
import { useAuth } from '../context/AuthContext.js';
import { ApiError, register } from '../lib/api.js';
import {
  validateDisplayName,
  validateEmail,
  validatePassword,
  validateUsername,
} from '../lib/validation.js';

// D-05: password bounds shown live via a client hint; the confirm-password field is
// client-only (the server has no such field).
function validateClient(fields: {
  email: string;
  username: string;
  password: string;
  confirmPassword: string;
  displayName: string;
}): Record<string, string[]> {
  const errors: Record<string, string[]> = {};
  const emailErrors = validateEmail(fields.email);
  if (emailErrors.length) errors.email = emailErrors;
  const usernameErrors = validateUsername(fields.username);
  if (usernameErrors.length) errors.username = usernameErrors;
  const passwordErrors = validatePassword(fields.password);
  if (passwordErrors.length) errors.password = passwordErrors;
  if (fields.displayName.trim() !== '') {
    const displayNameErrors = validateDisplayName(fields.displayName);
    if (displayNameErrors.length) errors.displayName = displayNameErrors;
  }
  if (fields.confirmPassword !== fields.password) {
    errors.confirmPassword = ['must match the password'];
  }
  return errors;
}

export default function Register() {
  const { setUser } = useAuth();
  const navigate = useNavigate();

  const [email, setEmail] = useState('');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [fieldErrors, setFieldErrors] = useState<Record<string, string[]>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setFormError(null);

    const clientErrors = validateClient({
      email,
      username,
      password,
      confirmPassword,
      displayName,
    });
    if (Object.keys(clientErrors).length > 0) {
      setFieldErrors(clientErrors);
      return;
    }
    setFieldErrors({});
    setSubmitting(true);
    try {
      const user = await register({
        email,
        username,
        password,
        displayName: displayName.trim() || undefined,
      });
      setUser(user);
      navigate('/', { replace: true });
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.details) {
          setFieldErrors(err.details);
        } else {
          setFormError(err.message);
        }
      } else {
        setFormError('Something went wrong. Please try again.');
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <AuthLayout
      title="Sign up"
      footer={
        <>
          Already have an account? <Link to="/login">Log in</Link>
        </>
      }
    >
      <form className="auth-form" onSubmit={handleSubmit} noValidate>
        {formError && (
          <p className="auth-form__error" role="alert">
            {formError}
          </p>
        )}
        <label className="auth-form__field">
          <span>Email</span>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
            required
          />
          {fieldErrors.email?.map((msg) => (
            <span key={msg} className="auth-form__field-error">
              {msg}
            </span>
          ))}
        </label>
        <label className="auth-form__field">
          <span>Username</span>
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoComplete="username"
            required
          />
          {fieldErrors.username?.map((msg) => (
            <span key={msg} className="auth-form__field-error">
              {msg}
            </span>
          ))}
        </label>
        <label className="auth-form__field">
          <span>Display name (optional)</span>
          <input
            type="text"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            autoComplete="name"
          />
          {fieldErrors.displayName?.map((msg) => (
            <span key={msg} className="auth-form__field-error">
              {msg}
            </span>
          ))}
        </label>
        <label className="auth-form__field">
          <span>Password (8-72 characters)</span>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="new-password"
            required
          />
          {fieldErrors.password?.map((msg) => (
            <span key={msg} className="auth-form__field-error">
              {msg}
            </span>
          ))}
        </label>
        <label className="auth-form__field">
          <span>Confirm password</span>
          <input
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            autoComplete="new-password"
            required
          />
          {fieldErrors.confirmPassword?.map((msg) => (
            <span key={msg} className="auth-form__field-error">
              {msg}
            </span>
          ))}
        </label>
        <button type="submit" className="auth-form__submit" disabled={submitting}>
          {submitting ? 'Signing up…' : 'Sign up'}
        </button>
      </form>
    </AuthLayout>
  );
}
