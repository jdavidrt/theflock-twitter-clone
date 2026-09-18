// Client mirror of server/internal/validation/validation.go (D-54). Only the constants,
// regexes and code-point counting need to match exactly — the server is the source of truth
// and re-validates everything; this just gives the user fast feedback before a round trip.

export const USERNAME_MIN_LENGTH = 3;
export const USERNAME_MAX_LENGTH = 20;
export const DISPLAY_NAME_MIN_LENGTH = 1;
export const DISPLAY_NAME_MAX_LENGTH = 50;
export const BIO_MAX_LENGTH = 160;
export const PASSWORD_MIN_LENGTH = 8;
export const PASSWORD_MAX_LENGTH = 72;
export const TWEET_CONTENT_MAX_LENGTH = 280;

export const USERNAME_REGEXP = /^[a-zA-Z0-9_]+$/;
// Intentionally simple (D-02: "RFC-ish"), matching the server's emailRegexp.
export const EMAIL_REGEXP = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

// countCodePoints matches the server's utf8.RuneCountInString (D-13): a spread respects
// surrogate pairs, so multi-byte characters (emoji, accented letters) count once each.
export function countCodePoints(s: string): number {
  return Array.from(s).length;
}

export function validateEmail(raw: string): string[] {
  const value = raw.trim().toLowerCase();
  return value === '' || !EMAIL_REGEXP.test(value) ? ['must be a valid email address'] : [];
}

export function validateUsername(raw: string): string[] {
  const value = raw.trim().toLowerCase();
  const errors: string[] = [];
  const length = countCodePoints(value);
  if (length < USERNAME_MIN_LENGTH || length > USERNAME_MAX_LENGTH) {
    errors.push(`must be between ${USERNAME_MIN_LENGTH} and ${USERNAME_MAX_LENGTH} characters`);
  }
  if (value !== '' && !USERNAME_REGEXP.test(value)) {
    errors.push('must contain only letters, numbers and underscores');
  }
  return errors;
}

export function validatePassword(raw: string): string[] {
  const length = countCodePoints(raw);
  return length < PASSWORD_MIN_LENGTH || length > PASSWORD_MAX_LENGTH
    ? [`must be between ${PASSWORD_MIN_LENGTH} and ${PASSWORD_MAX_LENGTH} characters`]
    : [];
}

export function validateDisplayName(raw: string): string[] {
  const value = raw.trim();
  const length = countCodePoints(value);
  return length < DISPLAY_NAME_MIN_LENGTH || length > DISPLAY_NAME_MAX_LENGTH
    ? [`must be between ${DISPLAY_NAME_MIN_LENGTH} and ${DISPLAY_NAME_MAX_LENGTH} characters`]
    : [];
}

export function validateBio(raw: string): string[] {
  const length = countCodePoints(raw.trim());
  return length > BIO_MAX_LENGTH ? [`must be at most ${BIO_MAX_LENGTH} characters`] : [];
}
