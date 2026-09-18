import './Avatar.css';

// D-12: no avatarUrl — the placeholder is the user's initials on a color deterministically
// derived from a hash of the username, so it needs no network and is stable across sessions.
const PALETTE = [
  '#1d9bf0',
  '#7856ff',
  '#00ba7c',
  '#f91880',
  '#ffad1f',
  '#f4212e',
  '#794bc4',
  '#0f9b8e',
];

function hashUsername(username: string): number {
  let hash = 0;
  for (let i = 0; i < username.length; i++) {
    hash = (hash * 31 + username.charCodeAt(i)) >>> 0;
  }
  return hash;
}

function initialsOf(displayName: string, username: string): string {
  const source = displayName.trim() || username;
  const parts = source.split(/\s+/).filter(Boolean);
  if (parts.length === 0) return '?';
  const first = parts[0].charAt(0);
  const last = parts.length > 1 ? parts[parts.length - 1].charAt(0) : '';
  return (first + last).toUpperCase();
}

interface AvatarProps {
  username: string;
  displayName: string;
  size?: number;
}

export default function Avatar({ username, displayName, size = 40 }: AvatarProps) {
  const background = PALETTE[hashUsername(username) % PALETTE.length];
  return (
    <div
      className="avatar"
      aria-hidden="true"
      style={{ width: size, height: size, fontSize: size * 0.4, backgroundColor: background }}
    >
      {initialsOf(displayName, username)}
    </div>
  );
}
