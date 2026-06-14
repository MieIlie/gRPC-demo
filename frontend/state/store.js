export const store = {
    currentUser: null,   // { id, username, displayName, token, refreshToken }
    activeRoom: null,    // { id, name, type }
    socketState: 'DISCONNECTED', // 'CONNECTING', 'CONNECTED', 'DISCONNECTED'
    onlineUsers: [],
    rooms: [],
    backendStatus: 'UNKNOWN',    // 'UNKNOWN', 'CHECKING', 'ONLINE', 'OFFLINE'
    backendLatency: null,        // number in ms
    traces: [],                  // list of inter-service trace events

    listeners: new Set(),

    subscribe(listener) {
        this.listeners.add(listener);
        return () => this.listeners.delete(listener);
    },

    setState(updates) {
        Object.assign(this, updates);
        this.listeners.forEach(listener => listener(this));
    }
};
