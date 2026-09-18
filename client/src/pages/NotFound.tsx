import './NotFound.css';

// Catches nav links to routes that land in a later step (e.g. /search, /:username before
// Step 9) and any truly unknown path, so navigation never dead-ends on a blank screen.
export default function NotFound() {
  return (
    <div className="not-found">
      <p>This page isn&apos;t built yet.</p>
    </div>
  );
}
