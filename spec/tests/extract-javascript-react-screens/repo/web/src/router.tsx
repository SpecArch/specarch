import { createBrowserRouter } from 'react-router-dom';
import { Layout } from './pages/Layout';
import NewVisitPage from './pages/NewVisitPage';
import { VisitsPage } from './pages/VisitsPage';
import { VisitorPage } from './pages/VisitorPage';
import { Settings } from 'some-design-system';

export const router = createBrowserRouter([
  {
    path: '/',
    element: <Layout />,
    children: [
      { index: true, element: <VisitsPage /> },
      { path: 'visits', element: <VisitsPage /> },
      { path: 'visits/new', element: <NewVisitPage /> },
      { path: 'visitors/:visitorId', Component: VisitorPage },
      { path: 'settings', element: <Settings /> },
      { path: 'files/*', element: <VisitsPage /> },
      { path: 'reports', lazy: () => import('./pages/Reports') },
    ],
  },
]);
