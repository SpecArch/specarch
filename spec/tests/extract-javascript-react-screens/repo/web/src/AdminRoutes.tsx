import { Route, Routes } from 'react-router-dom';
import { AdminPage } from './pages/AdminPage';

export function AdminRoutes({ beta }: { beta: boolean }) {
  return (
    <Routes>
      <Route path="/admin">
        <Route path="overview" element={<AdminPage />} />
      </Route>
      <Route path="/admin/overview" element={<AdminPage />} />
      {beta && <Route path="/beta" element={<AdminPage />} />}
    </Routes>
  );
}
