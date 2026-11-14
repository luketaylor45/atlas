import { lazy, Suspense } from 'react';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import ErrorBoundary from './components/ErrorBoundary';
import Layout from './components/Layout';
import RequireAuth from './components/RequireAuth';
import { FeedbackProvider, LoadingState } from './components/ui';
import { AuthProvider } from './context/AuthContext';
import { ThemeProvider } from './context/ThemeProvider';
import AdminLayout from './pages/admin/AdminLayout';
import LoginPage from './pages/auth/Login';
import RegisterPage from './pages/auth/Register';
import Dashboard from './pages/dashboard/Dashboard';
import MyServices from './pages/dashboard/MyServices';
import SettingsPage from './pages/dashboard/Settings';
import SetupPage from './pages/setup/Setup';

// The server page carries the terminal and code editor, and the admin pages
// are only used by administrators, so both load on demand.
const ServiceConsolePage = lazy(() => import('./pages/dashboard/ServiceConsole'));
const AdminOverviewPage = lazy(() => import('./pages/admin/Overview'));
const NodesPage = lazy(() => import('./pages/admin/Nodes'));
const CreateNodePage = lazy(() => import('./pages/admin/CreateNode'));
const AdminServicesPage = lazy(() => import('./pages/admin/Services'));
const CreateServicePage = lazy(() => import('./pages/admin/CreateService'));
const AdminEggsPage = lazy(() => import('./pages/admin/Eggs'));
const ImportEggPage = lazy(() => import('./pages/admin/ImportEgg'));
const AdminUsersPage = lazy(() => import('./pages/admin/Users'));
const AdminNewsPage = lazy(() => import('./pages/admin/News'));

function App() {
    return (
        <ThemeProvider>
            <ErrorBoundary>
                <FeedbackProvider>
                    <AuthProvider>
                        <BrowserRouter>
                            <Suspense fallback={<LoadingState />}>
                                <Routes>
                                    <Route path="/login" element={<LoginPage />} />
                                    <Route path="/register" element={<RegisterPage />} />
                                    <Route path="/setup" element={<SetupPage />} />

                                    <Route
                                        path="/"
                                        element={
                                            <RequireAuth>
                                                <Layout />
                                            </RequireAuth>
                                        }
                                    >
                                        <Route index element={<Dashboard />} />
                                        <Route path="services" element={<MyServices />} />
                                        <Route path="services/:uuid" element={<ServiceConsolePage />} />
                                        <Route path="settings" element={<SettingsPage />} />

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

                                    <Route path="*" element={<Navigate to="/" replace />} />
                                </Routes>
                            </Suspense>
                        </BrowserRouter>
                    </AuthProvider>
                </FeedbackProvider>
            </ErrorBoundary>
        </ThemeProvider>
    );
}

export default App;
