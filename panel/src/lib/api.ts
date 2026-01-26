import axios from 'axios';

const isProd = import.meta.env.PROD;

// In production, we assume Nginx handles the /api/v1 proxy on the standard port.
// If accessing directly via IP:3000, we need to explicitly point to :8080.
let base = `${window.location.protocol}//${window.location.hostname}`;
if (isProd) {
    if (window.location.port === '3000') {
        base += ':8080';
    }
    base += '/api/v1';
} else {
    base = 'http://localhost:8080/api/v1';
}

const api = axios.create({
    baseURL: base,
    headers: {
        'Content-Type': 'application/json',
    },
});

// Request interceptor to add token
api.interceptors.request.use((config) => {
    const token = localStorage.getItem('token');
    if (token) {
        config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
});

// Response interceptor to handle 401s
api.interceptors.response.use(
    (response) => response,
    (error) => {
        if (error.response?.status === 401) {
            localStorage.removeItem('token');
            // Optional: Redirect to login or window.location.href = '/login';
        }
        return Promise.reject(error);
    }
);

export default api;
