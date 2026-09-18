// Typed API client (D-42): every call is relative `/api/*` (Vite proxies it in dev),
// `credentials: 'include'` so the httpOnly session cookie (D-09) rides along, and every
// non-2xx response is unwrapped into an ApiError carrying the D-52 envelope's code/message/
// per-field details.

export interface User {
  id: string;
  username: string;
  email: string;
  displayName: string;
  bio: string;
  createdAt: string;
  updatedAt: string;
}

export interface ApiErrorBody {
  code: string;
  message: string;
  details?: Record<string, string[]>;
}

export class ApiError extends Error {
  status: number;
  code: string;
  details?: Record<string, string[]>;

  constructor(status: number, body: ApiErrorBody) {
    super(body.message);
    this.status = status;
    this.code = body.code;
    this.details = body.details;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  // D-56: every state-changing request must carry Content-Type: application/json, even one
  // with no body (POST /auth/logout) — the server checks the header unconditionally.
  const res = await fetch(`/api${path}`, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });

  if (res.status === 204) {
    return undefined as T;
  }

  const data = await res.json().catch(() => null);

  if (!res.ok) {
    const body: ApiErrorBody = data?.error ?? {
      code: 'INTERNAL',
      message: 'Something went wrong. Please try again.',
    };
    throw new ApiError(res.status, body);
  }

  return data as T;
}

export interface RegisterInput {
  email: string;
  username: string;
  password: string;
  displayName?: string;
}

export interface LoginInput {
  email: string;
  password: string;
}

export function register(input: RegisterInput): Promise<User> {
  return request<User>('/auth/register', { method: 'POST', body: JSON.stringify(input) });
}

export function login(input: LoginInput): Promise<User> {
  return request<User>('/auth/login', { method: 'POST', body: JSON.stringify(input) });
}

export function logout(): Promise<void> {
  return request<void>('/auth/logout', { method: 'POST' });
}

export function getMe(): Promise<User> {
  return request<User>('/auth/me');
}

export interface TweetAuthor {
  id: string;
  username: string;
  displayName: string;
}

// Mirrors server/internal/httpapi/tweets.go's tweetResponse: counts computed on read (D-23).
export interface Tweet {
  id: string;
  author: TweetAuthor;
  content: string;
  parentTweetId: string | null;
  createdAt: string;
  likeCount: number;
  replyCount: number;
  likedByMe: boolean;
}

export interface TweetPage {
  items: Tweet[];
  nextCursor: string | null;
}

export function getTimeline(cursor: string | null): Promise<TweetPage> {
  const params = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
  return request<TweetPage>(`/timeline${params}`);
}

export function createTweet(content: string): Promise<Tweet> {
  return request<Tweet>('/tweets', { method: 'POST', body: JSON.stringify({ content }) });
}

export function deleteTweet(id: string): Promise<void> {
  return request<void>(`/tweets/${id}`, { method: 'DELETE' });
}

export function likeTweet(id: string): Promise<Tweet> {
  return request<Tweet>(`/tweets/${id}/like`, { method: 'POST' });
}

export function unlikeTweet(id: string): Promise<Tweet> {
  return request<Tweet>(`/tweets/${id}/like`, { method: 'DELETE' });
}

export function getUserTweets(username: string, cursor: string | null): Promise<TweetPage> {
  const params = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
  return request<TweetPage>(`/users/${username}/tweets${params}`);
}

// Mirrors server/internal/httpapi/users.go's profileResponse: counts computed on read (D-23),
// no email (not a public profile field, D-12).
export interface Profile {
  id: string;
  username: string;
  displayName: string;
  bio: string;
  createdAt: string;
  tweetCount: number;
  followerCount: number;
  followingCount: number;
  isFollowedByMe: boolean;
}

export function getProfile(username: string): Promise<Profile> {
  return request<Profile>(`/users/${username}`);
}

// The server has no partial-update semantics despite the PATCH verb — both fields are always
// written, so callers that only mean to change one must pass the other's current value too.
export function updateProfile(input: { displayName: string; bio: string }): Promise<User> {
  return request<User>('/users/me', { method: 'PATCH', body: JSON.stringify(input) });
}

export function follow(username: string): Promise<Profile> {
  return request<Profile>(`/users/${username}/follow`, { method: 'POST' });
}

export function unfollow(username: string): Promise<Profile> {
  return request<Profile>(`/users/${username}/follow`, { method: 'DELETE' });
}

// D-24 followers/following row shape.
export interface FollowListItem {
  id: string;
  username: string;
  displayName: string;
  bio: string;
  isFollowedByMe: boolean;
}

export interface FollowListPage {
  items: FollowListItem[];
  nextCursor: string | null;
}

export function getFollowers(username: string, cursor: string | null): Promise<FollowListPage> {
  const params = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
  return request<FollowListPage>(`/users/${username}/followers${params}`);
}

export function getFollowing(username: string, cursor: string | null): Promise<FollowListPage> {
  const params = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
  return request<FollowListPage>(`/users/${username}/following${params}`);
}

export interface SearchResult {
  id: string;
  username: string;
  displayName: string;
}

// D-25: capped at 20, not paginated.
export function searchUsers(q: string): Promise<{ items: SearchResult[] }> {
  return request<{ items: SearchResult[] }>(`/search/users?q=${encodeURIComponent(q)}`);
}
