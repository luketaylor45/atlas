import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Layout from './components/Layout';
import LoginPage from './pages/auth/Login';
import RegisterPage from './pages/auth/Register';
import SetupPage from './pages/setup/Setup';
import Dashboard from './pages/dashboard/Dashboard';
import MyServices from './pages/dashboard/MyServices';
import ServiceConsolePage from './pages/dashboard/ServiceConsole';
import FileManager from './pages/dashboard/FileManager';
import AdminLayout from './pages/admin/AdminLayout';
import NodesPage from './pages/admin/Nodes';
import CreateNodePage from './pages/admin/CreateNode';
import AdminServicesPage from './pages/admin/Services';
import CreateServicePage from './pages/admin/CreateService';
import AdminOverviewPage from './pages/admin/Overview';
import AdminUsersPage from './pages/admin/Users';
import AdminNewsPage from './pages/admin/News';
import SettingsPage from './pages/dashboard/Settings';
import ImportEggPage from './pages/admin/ImportEgg';
import AdminEggsPage from './pages/admin/Eggs';
import { AuthProvider } from './context/AuthContext';

import RequireAuth from './components/RequireAuth';

function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
          <Route path="/setup" element={<SetupPage />} />

          <Route path="/" element={
            <RequireAuth>
              <Layout />
            </RequireAuth>
          }>
            <Route index element={<Dashboard />} />
            <Route path="services" element={<MyServices />} />
            <Route path="services/:uuid" element={<ServiceConsolePage />} />
            <Route path="services/:uuid/files" element={<FileManager />} />
            <Route path="settings" element={<SettingsPage />} />

            {/* Admin Routes */}
            <Route path="admin" element={<AdminLayout />}>
              <Route index element={<AdminOverviewPage />} />
              <Route path="nodes" element={<NodesPage />} />
              <Route path="nodes/create" element={<CreateNodePage />} />
              <Route path="services" element={<AdminServicesPage />} />
              <Route path="services/create" element={<CreateServicePage />} />
              <Route path="eggs" element={<AdminEggsPage />} />
              <Route path="eggs/import" element={<ImportEggPage />} />
              <Route path="users" element={<AdminUsersPage />} />
              <Route path="news" element={<AdminNewsPage />} />
            </Route>
          </Route>

          {/* Redirect unknown routes to dashboard if logged in, or login if not (fake logic for now) */}
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  );
}

export default App;
