import { store } from '../../state/store.js';
import { loadView, getElement } from './ui.js';
import { initSocket } from './socket.js';
import { initChat } from './chat.js';

export function checkAuth() {
    const token = localStorage.getItem('token');
    const user = localStorage.getItem('user');
    if (token && user) {
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

    let isRegisterMode = false;

    switchLink.addEventListener('click', (e) => {
        e.preventDefault();
        isRegisterMode = !isRegisterMode;
        if (isRegisterMode) {
            titleText.textContent = "Create Account";
            submitBtn.textContent = "Register";
            switchText.textContent = "Already have an account?";
            switchLink.textContent = "Login";
            displayNameGroup.style.display = "block";
        } else {
            titleText.textContent = "Welcome Back";
            submitBtn.textContent = "Login";
            switchText.textContent = "Don't have an account?";
            switchLink.textContent = "Register";
            displayNameGroup.style.display = "none";
        }
    });

    form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const username = getElement('username-input').value.trim();
        const password = getElement('password-input').value.trim();
        
        let url = '/api/auth/login';
        let body = { username, password };

        if (isRegisterMode) {
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
