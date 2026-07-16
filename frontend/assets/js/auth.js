import { store } from '../../state/store.js';
import { loadView, getElement } from './ui.js';
import { initSocket } from './socket.js';
import { initChat } from './chat.js';

function isTokenExpired(token) {
    if (!token) return true;
    try {
        const parts = token.split('.');
        if (parts.length !== 3) return true;
        const base64Url = parts[1];
        const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
        const jsonPayload = decodeURIComponent(atob(base64).split('').map(function(c) {
            return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2);
        }).join(''));
        const payload = JSON.parse(jsonPayload);
        if (!payload.exp) return false;
        const now = Math.floor(Date.now() / 1000);
        return now >= payload.exp;
    } catch (e) {
        console.error("Failed to parse JWT:", e);
        return true;
    }
}

export function checkAuth() {
    const token = localStorage.getItem('token');
    const user = localStorage.getItem('user');
    if (token && user) {
        if (isTokenExpired(token)) {
            console.warn("Session token expired. Clearing credentials and redirecting to login.");
            localStorage.removeItem('token');
            localStorage.removeItem('user');
            store.setState({ currentUser: null, activeRoom: null });
            navigateToLogin();
            return false;
        }
        const currentUser = JSON.parse(user);
        currentUser.token = token;
        store.setState({ currentUser });
        navigateToChat();
        return true;
    }
    navigateToLogin();
    return false;
}

export async function navigateToLogin() {
    await loadView('login');
    setupAuthListeners();
}

function setupAuthListeners() {
    const form = getElement('auth-form');
    const switchLink = getElement('auth-switch-link');
    const switchText = getElement('auth-switch-text');
    const titleText = getElement('auth-title-text');
    const submitBtn = getElement('auth-submit-btn');
    const displayNameGroup = getElement('display-name-group');
    const confirmPasswordGroup = getElement('confirm-password-group');
    const confirmPasswordInput = getElement('confirm-password-input');

    let isRegisterMode = false;

    // Subscribe to store connection state changes to update the diagnostic status badge and toggle submit button
    const unsubscribe = store.subscribe((state) => {
        const pill = getElement('login-conn-pill');
        if (!pill) {
            unsubscribe(); // Clean up listener when DOM is swapped
            return;
        }

        if (state.backendStatus === 'ONLINE') {
            pill.className = 'conn-pill online';
            pill.innerHTML = `<span class="status-dot"></span>Online (${state.backendLatency || 0}ms)`;
            submitBtn.disabled = false;
        } else if (state.backendStatus === 'CHECKING') {
            pill.className = 'conn-pill checking';
            pill.innerHTML = `<span class="status-dot"></span>Checking`;
            submitBtn.disabled = true;
        } else {
            pill.className = 'conn-pill offline';
            pill.innerHTML = `<span class="status-dot"></span>Offline`;
            submitBtn.disabled = true;
        }
    });

    switchLink.addEventListener('click', (e) => {
        e.preventDefault();
        isRegisterMode = !isRegisterMode;
        if (isRegisterMode) {
            titleText.textContent = "Create Account";
            submitBtn.textContent = "Register";
            switchText.textContent = "Already have an account?";
            switchLink.textContent = "Login";
            displayNameGroup.style.display = "block";
            confirmPasswordGroup.style.display = "block";
            confirmPasswordInput.required = true;
        } else {
            titleText.textContent = "Welcome Back";
            submitBtn.textContent = "Login";
            switchText.textContent = "Don't have an account?";
            switchLink.textContent = "Register";
            displayNameGroup.style.display = "none";
            confirmPasswordGroup.style.display = "none";
            confirmPasswordInput.required = false;
        }
    });

    form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const username = getElement('username-input').value.trim();
        const password = getElement('password-input').value.trim();
        
        let url = '/api/auth/login';
        let body = { username, password };

        if (isRegisterMode) {
            const confirmPassword = confirmPasswordInput.value.trim();
            if (password !== confirmPassword) {
                alert("Passwords do not match!");
                return;
            }
            url = '/api/auth/register';
            body.display_name = getElement('display-name-input').value.trim();
        }

        try {
            submitBtn.disabled = true;
            submitBtn.textContent = isRegisterMode ? "Registering..." : "Logging in...";
            
            const res = await fetch(url, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body)
            });

            if (!res.ok) {
                const errMsg = await res.text();
                throw new Error(errMsg || "Authentication failed");
            }

            const data = await res.json();
            
            localStorage.setItem('token', data.token);
            localStorage.setItem('user', JSON.stringify({
                id: data.user_id,
                username: data.username,
                displayName: data.display_name
            }));

            store.setState({
                currentUser: {
                    id: data.user_id,
                    username: data.username,
                    displayName: data.display_name,
                    token: data.token
                }
            });

            navigateToChat();
        } catch (err) {
            alert(err.message);
            submitBtn.disabled = false;
            submitBtn.textContent = isRegisterMode ? "Register" : "Login";
        }
    });
}

export async function navigateToChat() {
    await loadView('chat');
    initSocket();
    initChat();
}

export function logout() {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    store.setState({ currentUser: null, activeRoom: null });
    navigateToLogin();
}
