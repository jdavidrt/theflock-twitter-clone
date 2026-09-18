import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import Avatar from '../components/Avatar.js';
import { searchUsers } from '../lib/api.js';
import './Search.css';

const DEBOUNCE_MS = 300;

export default function Search() {
  const [query, setQuery] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');

  // D-26: 300ms debounce, no search history or suggestions.
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query.trim()), DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [query]);

  const { data, isLoading, isError } = useQuery({
    queryKey: ['search', debouncedQuery],
    queryFn: () => searchUsers(debouncedQuery),
    enabled: debouncedQuery.length > 0,
  });

  const results = data?.items ?? [];

  return (
    <div className="search">
      <input
        type="search"
        className="search__input"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Search people"
        aria-label="Search people"
        autoFocus
      />

      {debouncedQuery.length === 0 && <p className="search__status">Search by username or name.</p>}
      {debouncedQuery.length > 0 && isLoading && <p className="search__status">Searching…</p>}
      {isError && (
        <p className="search__status search__status--error">Something went wrong. Try again.</p>
      )}
      {debouncedQuery.length > 0 && !isLoading && !isError && results.length === 0 && (
        <p className="search__status">No people found for "{debouncedQuery}".</p>
      )}

      <ul className="search__results">
        {results.map((result) => (
          <li key={result.id}>
            <Link to={`/${result.username}`} className="search__result">
              <Avatar username={result.username} displayName={result.displayName} />
              <span className="search__names">
                <span className="search__display-name">{result.displayName}</span>
                <span className="search__username">@{result.username}</span>
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
