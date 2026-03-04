import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import AppShell from './components/layout/AppShell';
import LayersPage from './pages/LayersPage';
import SegmentPage from './pages/SegmentPage';
import TestingPage from './pages/TestingPage';
import ImportExportPage from './pages/ImportExportPage';

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: false } },
});

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route element={<AppShell />}>
            <Route path="/layers" element={<LayersPage />} />
            <Route path="/layers/:name/segments/:id" element={<SegmentPage />} />
            <Route path="/testing" element={<TestingPage />} />
            <Route path="/config" element={<ImportExportPage />} />
            <Route path="*" element={<Navigate to="/layers" replace />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
