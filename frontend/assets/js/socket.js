import { store } from '../../state/store.js';

let ws = null;
let reconnectTimeout = null;

export function initSocket() {
    const user = store.currentUser;
    if (!user || !user.token) return;

    if (ws) {
        ws.close();
    }

    const loc = window.location;
    const proto = loc.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${proto}//${loc.host}/ws?token=${user.token}`;

    store.setState({ socketState: 'CONNECTING' });

    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
        console.log("WebSocket connected");
        store.setState({ socketState: 'CONNECTED' });
        if (reconnectTimeout) {
            clearTimeout(reconnectTimeout);
            reconnectTimeout = null;
        }
    };

    ws.onmessage = (event) => {
        console.log("WS Received:", event.data);
        
        try {
            const data = JSON.parse(event.data);
            handleWSMessage(data);
        } catch (err) {
            console.log("Non-JSON text message from server:", event.data);
        }
    };

    ws.onclose = () => {
        console.log("WebSocket closed");
        store.setState({ socketState: 'DISCONNECTED' });
        
        if (!reconnectTimeout && store.currentUser) {
            reconnectTimeout = setTimeout(() => {
                reconnectTimeout = null;
                initSocket();
            }, 5000);
        }
    };

    ws.onerror = (err) => {
        console.error("WebSocket error:", err);
    };
}

export function sendWSMessage(event, data) {
    if (ws && ws.readyState === WebSocket.OPEN) {
        const payload = JSON.stringify({ event, data });
        ws.send(payload);
        return true;
    }
    console.error("Cannot send WS message, socket not open");
    return false;
}

function handleWSMessage(msg) {
    const listeners = socketEventListeners[msg.event] || [];
    listeners.forEach(cb => cb(msg.data));
}

const socketEventListeners = {};

export function registerSocketListener(event, callback) {
    if (!socketEventListeners[event]) {
        socketEventListeners[event] = [];
    }
    socketEventListeners[event].push(callback);
}
