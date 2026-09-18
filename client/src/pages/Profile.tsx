import { useState } from 'react';
import type { FormEvent } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import Avatar from '../components/Avatar.js';
import TweetList from '../components/TweetList.js';
import { useAuth } from '../context/AuthContext.js';
import {
  ApiError,
  follow,
  getProfile,
  getUserTweets,
  unfollow,
  updateProfile,
} from '../lib/api.js';
import type { Profile as ProfileData } from '../lib/api.js';
import { validateBio } from '../lib/validation.js';
import { useTweetFeed } from '../lib/useTweetFeed.js';
import './Profile.css';

export default function Profile() {
  const { username = '' } = useParams();
  const { user: currentUser } = useAuth();
  const queryClient = useQueryClient();
  const profileKey = ['profile', username];

  const {
    data: profile,
    isLoading,
    isError,
  } = useQuery({ queryKey: profileKey, queryFn: () => getProfile(username) });

  const [editingBio, setEditingBio] = useState(false);
  const [bioDraft, setBioDraft] = useState('');
  const [bioError, setBioError] = useState<string | null>(null);
  const [savingBio, setSavingBio] = useState(false);

  const followMutation = useMutation({
    mutationFn: () => (profile?.isFollowedByMe ? unfollow(username) : follow(username)),
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: profileKey });
      const previous = queryClient.getQueryData<ProfileData>(profileKey);
      queryClient.setQueryData<ProfileData>(profileKey, (old) =>
        old
          ? {
              ...old,
              isFollowedByMe: !old.isFollowedByMe,
              followerCount: old.followerCount + (old.isFollowedByMe ? -1 : 1),
            }
          : old,
      );
      return { previous };
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(profileKey, context.previous);
    },
    onSuccess: (updated) => {
      queryClient.setQueryData(profileKey, updated);
    },
  });

  const feed = useTweetFeed({
    queryKey: ['userTweets', username],
    fetchPage: (cursor) => getUserTweets(username, cursor),
  });

  function startEditingBio() {
    if (!profile) return;
    setBioDraft(profile.bio);
    setBioError(null);
    setEditingBio(true);
  }

  async function handleBioSubmit(e: FormEvent) {
    e.preventDefault();
    if (!profile) return;
    const errors = validateBio(bioDraft);
    if (errors.length > 0) {
      setBioError(errors[0]);
      return;
    }
    setSavingBio(true);
    setBioError(null);
    try {
      const trimmed = bioDraft.trim();
      await updateProfile({ displayName: profile.displayName, bio: trimmed });
      queryClient.setQueryData<ProfileData>(profileKey, (old) =>
        old ? { ...old, bio: trimmed } : old,
      );
      setEditingBio(false);
    } catch (err) {
      setBioError(
        err instanceof ApiError ? err.message : 'Something went wrong. Please try again.',
      );
    } finally {
      setSavingBio(false);
    }
  }

  if (isLoading) return <p className="profile__status">Loading profile…</p>;
  if (isError || !profile) {
    return <p className="profile__status profile__status--error">User not found.</p>;
  }

  const isOwnProfile = currentUser?.username === profile.username;

  return (
    <div className="profile">
      <header className="profile__header">
        <Avatar username={profile.username} displayName={profile.displayName} size={72} />
        <h1 className="profile__display-name">{profile.displayName}</h1>
        <p className="profile__username">@{profile.username}</p>

        {!editingBio && profile.bio && <p className="profile__bio">{profile.bio}</p>}

        {!editingBio && isOwnProfile && (
          <button type="button" className="profile__edit-bio-trigger" onClick={startEditingBio}>
            {profile.bio ? 'Edit bio' : 'Add a bio'}
          </button>
        )}

        {editingBio && (
          <form className="profile__bio-form" onSubmit={handleBioSubmit}>
            <label className="profile__bio-label" htmlFor="bio">
              Bio
            </label>
            <textarea
              id="bio"
              value={bioDraft}
              onChange={(e) => setBioDraft(e.target.value)}
              rows={2}
            />
            {bioError && (
              <p className="profile__bio-error" role="alert">
                {bioError}
              </p>
            )}
            <div className="profile__bio-actions">
              <button
                type="button"
                className="profile__bio-cancel"
                onClick={() => setEditingBio(false)}
                disabled={savingBio}
              >
                Cancel
              </button>
              <button type="submit" className="profile__bio-save" disabled={savingBio}>
                {savingBio ? 'Saving…' : 'Save'}
              </button>
            </div>
          </form>
        )}

        <div className="profile__counts">
          <Link to={`/${profile.username}/following`} className="profile__count">
            <strong>{profile.followingCount}</strong> Following
          </Link>
          <Link to={`/${profile.username}/followers`} className="profile__count">
            <strong>{profile.followerCount}</strong> Followers
          </Link>
        </div>

        {!isOwnProfile && (
          <button
            type="button"
            className={
              'profile__follow-btn' +
              (profile.isFollowedByMe ? ' profile__follow-btn--following' : '')
            }
            onClick={() => followMutation.mutate()}
            disabled={followMutation.isPending}
          >
            {profile.isFollowedByMe ? 'Following' : 'Follow'}
          </button>
        )}
      </header>

      <TweetList
        tweets={feed.tweets}
        isLoading={feed.isLoading}
        isError={feed.isError}
        emptyState={<p className="profile__empty">No tweets yet.</p>}
        hasNextPage={feed.hasNextPage}
        isFetchingNextPage={feed.isFetchingNextPage}
        fetchNextPage={feed.fetchNextPage}
        sentinelRef={feed.sentinelRef}
        currentUsername={currentUser?.username ?? ''}
        onLikeToggle={(tweet) => feed.likeMutation.mutate(tweet)}
        onDelete={(id) => feed.deleteMutation.mutate(id)}
      />
    </div>
  );
}
