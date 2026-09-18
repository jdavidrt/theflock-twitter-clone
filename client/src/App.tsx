import { Route, Routes } from 'react-router-dom';
import AppShell from './components/AppShell.js';
import ProtectedRoute from './components/ProtectedRoute.js';
import PublicOnlyRoute from './components/PublicOnlyRoute.js';
import FollowList from './pages/FollowList.js';
import Home from './pages/Home.js';
import Login from './pages/Login.js';
import NotFound from './pages/NotFound.js';
import Profile from './pages/Profile.js';
import Register from './pages/Register.js';
import Search from './pages/Search.js';

export default function App() {
  return (
    <Routes>
      <Route
        path="/login"
        element={
          <PublicOnlyRoute>
            <Login />
          </PublicOnlyRoute>
        }
      />
      <Route
        path="/register"
        element={
          <PublicOnlyRoute>
            <Register />
          </PublicOnlyRoute>
        }
      />
      <Route
        path="/*"
        element={
          <ProtectedRoute>
            <AppShell>
              <Routes>
                <Route path="/" element={<Home />} />
                <Route path="/search" element={<Search />} />
                <Route path="/:username" element={<Profile />} />
                <Route path="/:username/followers" element={<FollowList mode="followers" />} />
                <Route path="/:username/following" element={<FollowList mode="following" />} />
                <Route path="*" element={<NotFound />} />
              </Routes>
            </AppShell>
          </ProtectedRoute>
        }
      />
    </Routes>
  );
}
