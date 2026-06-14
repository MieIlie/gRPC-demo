import { store } from '../../state/store.js';

export async function checkBackendConnection() {
    store.setState({ backendStatus: 'CHECKING' });
    const start = Date.now();
    try {
        const res = await fetch('/', { cache: 'no-store' });
        if (res.ok) {
            const latency = Date.now() - start;
            store.setState({ 
                backendStatus: 'ONLINE',
                backendLatency: latency
            });
            return true;
        }
    } catch (err) {
        console.error("Backend connection check failed:", err);
    }
    store.setState({ 
        backendStatus: 'OFFLINE',
        backendLatency: null
    });

    // If backend is offline, expire all session storage credentials and force redirect to login
    if (localStorage.getItem('token') || store.currentUser) {
        console.warn("Backend connection unavailable. Expiring session and redirecting to login.");
        localStorage.removeItem('token');
        localStorage.removeItem('user');
        store.setState({ currentUser: null, activeRoom: null });
        
        // Dynamic import to avoid static import coupling and cycle
        const loginCard = document.getElementById('auth-card-view');
        if (!loginCard) {
            import('./auth.js').then(auth => auth.navigateToLogin());
        }
    }
    
    return false;
}

// Set up automatic polling every 10 seconds to detect server state transitions
export function startConnectionPolling() {
    checkBackendConnection();
    return setInterval(checkBackendConnection, 10000);
}
