import { store } from '../../state/store.js';
import { getElement } from './ui.js';
import { logout } from './auth.js';
import { sendWSMessage, registerSocketListener } from './socket.js';
import { clearTraces } from './traces.js';

let isTypingSent = false;
let typingTimeout = null;
let activeTypingUsers = {};

export function initChat() {
    setupChatDOM();
    setupChatListeners();
    registerSocketHandlers();

    // Force select General Room on init to direct user straight to a room
    const defaultRoom = { id: 'a3333333-3333-3333-3333-333333333333', name: 'General Room', type: 'group' };
    store.setState({ activeRoom: defaultRoom });

    // Load rooms list and messages in parallel
    fetchRooms();
    fetchRoomMessages(defaultRoom.id);
}

function setupChatDOM() {
    const user = store.currentUser;
    if (user) {
        getElement('current-user-name').textContent = user.displayName || user.username;
        getElement('current-user-avatar').textContent = (user.displayName || user.username).substring(0, 1).toUpperCase();
    }
}

function setupChatListeners() {
    const logoutBtn = getElement('logout-btn');
    const sendBtn = getElement('chat-send-btn');
    const input = getElement('chat-message-input');
    const createRoomBtn = getElement('create-room-btn');

    // Tab buttons and containers
    const chatTabBtn = getElement('chat-tab-btn');
    const monitorTabBtn = getElement('monitor-tab-btn');
    const chatTabContent = getElement('chat-tab-content');
    const monitorTabContent = getElement('monitor-tab-content');

    if (chatTabBtn && monitorTabBtn) {
        chatTabBtn.addEventListener('click', () => {
            chatTabBtn.classList.add('active');
            monitorTabBtn.classList.remove('active');
            chatTabContent.style.display = 'flex';
            monitorTabContent.style.display = 'none';
        });

        monitorTabBtn.addEventListener('click', () => {
            monitorTabBtn.classList.add('active');
            chatTabBtn.classList.remove('active');
            chatTabContent.style.display = 'none';
            monitorTabContent.style.display = 'flex';
            // Refresh logs rendering on tab selection
            renderTraces(store.traces);
        });
    }

    const clearTracesBtn = getElement('clear-traces-btn');
    if (clearTracesBtn) {
        clearTracesBtn.addEventListener('click', () => {
            clearTraces();
        });
    }

    logoutBtn.addEventListener('click', () => {
        logout();
    });

    createRoomBtn.addEventListener('click', async () => {
        const name = prompt("Enter room name:");
        if (!name) return;

        try {
            const token = localStorage.getItem('token');
            const res = await fetch('/api/rooms', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify({
                    room_type: 1, // GROUP
                    room_name: name,
                    member_ids: [] // Creator is auto-added
                })
            });

            if (!res.ok) {
                throw new Error(`Failed to create room: ${res.statusText}`);
            }

            // Re-fetch all rooms to get the updated list from DB
            await fetchRooms();
        } catch (err) {
            console.error("Error creating room:", err);
            alert("Error creating room: " + err.message);
        }
    });

    sendBtn.addEventListener('click', () => {
        sendMessage();
    });

    input.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            sendMessage();
        }
    });

    input.addEventListener('input', () => {
        sendTypingIndicator();
    });

    const attachBtn = getElement('chat-attach-btn');
    const fileInput = getElement('chat-video-upload');

    if (attachBtn && fileInput) {
        attachBtn.addEventListener('click', () => {
            fileInput.click();
        });

        fileInput.addEventListener('change', async () => {
            const file = fileInput.files[0];
            if (!file) return;
            await uploadVideoFile(file);
            fileInput.value = '';
        });
    }

    let lastTracesLength = 0;

    const unsubscribe = store.subscribe((state) => {
        const activeRoomTitle = getElement('active-room-title');
        if (!activeRoomTitle) {
            unsubscribe();
            return;
        }

        const startCallBtn = getElement('start-call-btn');
        const attachBtn = getElement('chat-attach-btn');
        if (state.activeRoom) {
            input.disabled = false;
            sendBtn.disabled = false;
            if (attachBtn) attachBtn.disabled = false;
            activeRoomTitle.textContent = state.activeRoom.name;
            if (startCallBtn) {
                startCallBtn.style.display = 'block';
            }
        } else {
            input.disabled = true;
            sendBtn.disabled = true;
            if (attachBtn) attachBtn.disabled = true;
            activeRoomTitle.textContent = "Select or Create a Room";
            if (startCallBtn) {
                startCallBtn.style.display = 'none';
            }
        }

        // Live connection indicator updates (sidebar and nodes)
        const gatewayStatusEl = getElement('sidebar-gateway-status');
        if (gatewayStatusEl) {
            if (state.backendStatus === 'ONLINE') {
                gatewayStatusEl.className = 'sidebar-status-value online';
                gatewayStatusEl.innerHTML = `<span class="status-dot"></span>Online (${state.backendLatency || 0}ms)`;
                
                const gwNodeStatus = getElement('status-gateway');
                if (gwNodeStatus) {
                    gwNodeStatus.textContent = `Online (${state.backendLatency || 0}ms)`;
                    gwNodeStatus.style.color = 'var(--success)';
                }
            } else if (state.backendStatus === 'CHECKING') {
                gatewayStatusEl.className = 'sidebar-status-value connecting';
                gatewayStatusEl.innerHTML = `<span class="status-dot"></span>Checking...`;
            } else {
                gatewayStatusEl.className = 'sidebar-status-value offline';
                gatewayStatusEl.innerHTML = `<span class="status-dot"></span>Offline`;
                
                const gwNodeStatus = getElement('status-gateway');
                if (gwNodeStatus) {
                    gwNodeStatus.textContent = `Offline`;
                    gwNodeStatus.style.color = 'var(--error)';
                }
            }
        }

        const socketStatusEl = getElement('sidebar-websocket-status');
        if (socketStatusEl) {
            const clientWSNodeStatus = getElement('status-client-ws');
            const connLineClientGW = getElement('conn-client-gateway');
            
            if (state.socketState === 'CONNECTED') {
                socketStatusEl.className = 'sidebar-status-value online';
                socketStatusEl.innerHTML = `<span class="status-dot"></span>Connected`;
                
                if (clientWSNodeStatus) {
                    clientWSNodeStatus.textContent = 'WS: Connected';
                    clientWSNodeStatus.classList.add('connected');
                }
                if (connLineClientGW) {
                    connLineClientGW.style.borderColor = 'var(--secondary)';
                }
            } else if (state.socketState === 'CONNECTING') {
                socketStatusEl.className = 'sidebar-status-value connecting';
                socketStatusEl.innerHTML = `<span class="status-dot"></span>Connecting...`;
                
                if (clientWSNodeStatus) {
                    clientWSNodeStatus.textContent = 'WS: Connecting';
                    clientWSNodeStatus.classList.remove('connected');
                }
            } else {
                socketStatusEl.className = 'sidebar-status-value offline';
                socketStatusEl.innerHTML = `<span class="status-dot"></span>Disconnected`;
                
                if (clientWSNodeStatus) {
                    clientWSNodeStatus.textContent = 'WS: Offline';
                    clientWSNodeStatus.classList.remove('connected');
                }
                if (connLineClientGW) {
                    connLineClientGW.style.borderColor = 'var(--border-light)';
                }
            }
        }

        // Trace updates and network graph flashes
        if (state.traces.length !== lastTracesLength) {
            if (state.traces.length > lastTracesLength && state.traces.length > 0) {
                const latestTrace = state.traces[0];
                flashNetworkLink(latestTrace.source, latestTrace.target);
            }
            lastTracesLength = state.traces.length;
            
            if (monitorTabContent && monitorTabContent.style.display !== 'none') {
                renderTraces(state.traces);
            }
        }
    });

    renderRooms();
}

function renderRooms() {
    const listEl = getElement('rooms-list');
    if (!listEl) return;
    listEl.innerHTML = '';

    const defaultRoom = { id: 'a3333333-3333-3333-3333-333333333333', name: 'General Room', type: 'group' };
    const hasGeneralRoom = (store.rooms || []).some(r => r.id === defaultRoom.id);
    const allRooms = hasGeneralRoom ? store.rooms : [defaultRoom, ...store.rooms];

    allRooms.forEach(room => {
        const item = document.createElement('div');
        item.className = 'room-item';
        if (store.activeRoom && store.activeRoom.id === room.id) {
            item.className += ' active';
        }
        item.textContent = room.name;
        item.addEventListener('click', () => {
            store.setState({ activeRoom: room });
            renderRooms();
            fetchRoomMessages(room.id);
        });
        listEl.appendChild(item);
    });
}

async function fetchRooms() {
    try {
        const token = localStorage.getItem('token');
        const res = await fetch('/api/rooms', {
            headers: {
                'Authorization': `Bearer ${token}`
            }
        });

        if (!res.ok) {
            throw new Error(`Failed to fetch rooms: ${res.statusText}`);
        }

        const data = await res.json();
        const mappedRooms = (data.rooms || []).map(r => {
            const roomType = r.room_type !== undefined ? r.room_type : r.roomType;
            const roomName = r.room_name !== undefined ? r.room_name : r.roomName;
            const isDirect = (roomType === 0 || roomType === 'DIRECT' || roomType === 'RoomType_DIRECT' || String(roomType).toUpperCase() === 'DIRECT');
            return {
                id: r.id,
                name: roomName || (isDirect ? 'Direct Message' : 'Group Room'),
                type: isDirect ? 'direct' : 'group'
            };
        });

        store.setState({ rooms: mappedRooms });
        renderRooms();
    } catch (err) {
        console.error("Error fetching rooms:", err);
    }
}

async function fetchRoomMessages(roomId) {
    const container = getElement('messages-container');
    if (!container) return;

    container.innerHTML = `
        <div style="text-align: center; color: var(--text-muted); margin-top: 2rem;">
            Loading messages...
        </div>
    `;

    try {
        const token = localStorage.getItem('token');
        const res = await fetch(`/api/rooms/${roomId}/messages?limit=50`, {
            headers: {
                'Authorization': `Bearer ${token}`
            }
        });

        if (!res.ok) {
            throw new Error(`Failed to fetch message history: ${res.statusText}`);
        }

        const data = await res.json();
        
        container.innerHTML = '';
        
        if (!data.messages || data.messages.length === 0) {
            container.innerHTML = `
                <div style="text-align: center; color: var(--text-muted); margin-top: 2rem;">
                    No messages yet. Send a message to start!
                </div>
            `;
            return;
        }

        // Reversing list because backend queries order by created_at DESC (newest first).
        const chronologicalMessages = [...data.messages].reverse();

        chronologicalMessages.forEach(msg => {
            const isOutgoing = msg.sender_id === store.currentUser.id;
            const bubble = document.createElement('div');
            bubble.className = `message-bubble ${isOutgoing ? 'outgoing' : 'incoming'}`;
            
            let timeStr = 'now';
            if (msg.created_at) {
                const date = new Date(msg.created_at);
                if (!isNaN(date.getTime())) {
                    timeStr = date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
                }
            }
            
            const senderName = isOutgoing ? 'You' : (msg.sender_id === '11111111-1111-1111-1111-111111111111' ? 'Alice' : (msg.sender_id === '22222222-2222-2222-2222-222222222222' ? 'Bob' : 'User'));

            let contentHTML = msg.content;
            if (msg.content && (msg.content.startsWith('/uploads/') || msg.content.endsWith('.mp4') || msg.content.endsWith('.webm') || msg.content.endsWith('.ogg') || msg.content.endsWith('.mov') || msg.content.includes('/uploads/'))) {
                contentHTML = `
                    <div style="margin-top: 0.5rem; max-width: 320px; border-radius: 8px; overflow: hidden; background: #000; aspect-ratio: 16/9;">
                        <video src="${msg.content}" controls style="width: 100%; height: 100%; object-fit: cover;"></video>
                    </div>
                `;
            } else {
                contentHTML = `<div>${msg.content}</div>`;
            }

            bubble.innerHTML = `
                ${contentHTML}
                <div class="message-meta">
                    <span>${senderName}</span>
                    <span>${timeStr}</span>
                </div>
            `;
            container.appendChild(bubble);
        });

        container.scrollTop = container.scrollHeight;
    } catch (err) {
        console.error("Error fetching message history:", err);
        container.innerHTML = `
            <div style="text-align: center; color: var(--error); margin-top: 2rem;">
                Error loading message history.
            </div>
        `;
    }
}

function sendTypingIndicator() {
    if (!store.activeRoom) return;
    if (isTypingSent) return;

    isTypingSent = true;
    sendWSMessage('chat.typing', {
        roomId: store.activeRoom.id
    });

    setTimeout(() => {
        isTypingSent = false;
    }, 2000);
}

function renderOnlineUsers() {
    const listEl = getElement('online-users-list');
    if (!listEl) return;
    listEl.innerHTML = '';

    const online = store.onlineUsers || [];
    if (online.length === 0) {
        listEl.innerHTML = '<div style="color: var(--text-muted); font-size: 0.85rem; padding: 0.5rem;">No users online</div>';
        return;
    }

    online.forEach(uID => {
        const isSelf = store.currentUser && store.currentUser.id === uID;
        const name = isSelf ? 'You' : (uID === '11111111-1111-1111-1111-111111111111' ? 'Alice Henderson' : (uID === '22222222-2222-2222-2222-222222222222' ? 'Bob Vance' : 'User'));
        
        const item = document.createElement('div');
        item.className = 'room-item';
        item.style.display = 'flex';
        item.style.alignItems = 'center';
        item.style.gap = '0.5rem';
        
        if (isSelf) {
            item.style.cursor = 'default';
        } else {
            item.style.cursor = 'pointer';
            item.title = `Click to message ${name}`;
            item.addEventListener('click', () => {
                startDirectMessage(uID, name);
            });
        }

        item.innerHTML = `
            <span class="status-dot" style="background: var(--success); width: 8px; height: 8px; border-radius: 50%; display: inline-block;"></span>
            <span>${name}</span>
        `;
        listEl.appendChild(item);
    });
}

async function startDirectMessage(targetUserId, displayName) {
    try {
        const token = localStorage.getItem('token');
        const res = await fetch('/api/rooms', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify({
                room_type: 0, // DIRECT
                room_name: `DM with ${displayName}`,
                member_ids: [targetUserId]
            })
        });

        if (!res.ok) {
            throw new Error(`Failed to start DM: ${res.statusText}`);
        }

        const data = await res.json();
        const room = data.room;

        // Re-fetch all rooms
        await fetchRooms();

        // Switch to the DM room
        const targetRoom = {
            id: room.id,
            name: room.room_name || room.roomName || `DM with ${displayName}`,
            type: 'direct'
        };
        store.setState({ activeRoom: targetRoom });
        renderRooms();
        fetchRoomMessages(room.id);
    } catch (err) {
        console.error("Error starting direct message:", err);
        alert("Error starting DM: " + err.message);
    }
}

function updateTypingUI() {
    const indicatorEl = getElement('room-typing-indicator');
    if (!indicatorEl) return;

    const typers = Object.keys(activeTypingUsers).filter(id => activeTypingUsers[id] === true);
    if (typers.length === 0) {
        indicatorEl.textContent = '';
        return;
    }

    const names = typers.map(id => id === '11111111-1111-1111-1111-111111111111' ? 'Alice' : (id === '22222222-2222-2222-2222-222222222222' ? 'Bob' : 'Someone'));
    if (names.length === 1) {
        indicatorEl.textContent = `${names[0]} is typing...`;
    } else {
        indicatorEl.textContent = `${names.join(', ')} are typing...`;
    }
}

function sendMessage() {
    const input = getElement('chat-message-input');
    const content = input.value.trim();
    if (content === "" || !store.activeRoom) return;

    sendWSMessage('chat.send', {
        roomId: store.activeRoom.id,
        content: content
    });

    input.value = '';
}

async function uploadVideoFile(file) {
    const attachBtn = getElement('chat-attach-btn');
    if (!attachBtn) return;
    const originalText = attachBtn.textContent;
    attachBtn.textContent = '⏳';
    attachBtn.disabled = true;

    try {
        const formData = new FormData();
        formData.append('file', file);

        const token = localStorage.getItem('token');
        const res = await fetch('/api/upload', {
            method: 'POST',
            headers: {
                'Authorization': `Bearer ${token}`
            },
            body: formData
        });

        if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || `Upload failed: ${res.statusText}`);
        }

        const data = await res.json();
        const videoURL = data.url;

        // Send the video message over WebSocket!
        sendWSMessage('chat.send', {
            roomId: store.activeRoom.id,
            content: videoURL
        });

    } catch (err) {
        console.error("Error uploading video file:", err);
        alert("Upload failed: " + err.message);
    } finally {
        attachBtn.textContent = originalText;
        attachBtn.disabled = false;
    }
}

function registerSocketHandlers() {
    registerSocketListener('chat.receive', (data) => {
        const container = getElement('messages-container');
        if (!container) return;
        
        if (container.querySelector('div[style*="text-align: center"]')) {
            container.innerHTML = '';
        }

        const isOutgoing = data.senderId === store.currentUser.id;

        const bubble = document.createElement('div');
        bubble.className = `message-bubble ${isOutgoing ? 'outgoing' : 'incoming'}`;
        
        const senderName = isOutgoing ? 'You' : (data.senderId === '11111111-1111-1111-1111-111111111111' ? 'Alice' : (data.senderId === '22222222-2222-2222-2222-222222222222' ? 'Bob' : 'User'));

        let contentHTML = data.content;
        if (data.content && (data.content.startsWith('/uploads/') || data.content.endsWith('.mp4') || data.content.endsWith('.webm') || data.content.endsWith('.ogg') || data.content.endsWith('.mov') || data.content.includes('/uploads/'))) {
            contentHTML = `
                <div style="margin-top: 0.5rem; max-width: 320px; border-radius: 8px; overflow: hidden; background: #000; aspect-ratio: 16/9;">
                    <video src="${data.content}" controls style="width: 100%; height: 100%; object-fit: cover;"></video>
                </div>
            `;
        } else {
            contentHTML = `<div>${data.content}</div>`;
        }

        bubble.innerHTML = `
            ${contentHTML}
            <div class="message-meta">
                <span>${senderName}</span>
                <span>${data.createdAt || 'now'}</span>
            </div>
        `;

        container.appendChild(bubble);
        container.scrollTop = container.scrollHeight;
    });

    registerSocketListener('chat.typing', (data) => {
        if (!store.activeRoom || store.activeRoom.id !== data.roomId) return;

        activeTypingUsers[data.userId] = true;
        updateTypingUI();

        // Clear typing indicator for this user after 3 seconds of inactivity
        if (typingTimeout) {
            clearTimeout(typingTimeout);
        }
        typingTimeout = setTimeout(() => {
            activeTypingUsers[data.userId] = false;
            updateTypingUI();
        }, 3000);
    });

    registerSocketListener('user.online_list', (userIds) => {
        store.setState({ onlineUsers: userIds || [] });
        renderOnlineUsers();
    });

    registerSocketListener('user.online', (data) => {
        const current = store.onlineUsers || [];
        if (!current.includes(data.userId)) {
            const updated = [...current, data.userId];
            store.setState({ onlineUsers: updated });
            renderOnlineUsers();
        }
    });

    registerSocketListener('user.offline', (data) => {
        const current = store.onlineUsers || [];
        const updated = current.filter(id => id !== data.userId);
        store.setState({ onlineUsers: updated });
        renderOnlineUsers();
        
        // Also remove from typing users if they go offline
        if (activeTypingUsers[data.userId]) {
            delete activeTypingUsers[data.userId];
            updateTypingUI();
        }
    });
}

function renderTraces(traces) {
    const listEl = getElement('traces-list');
    if (!listEl) return;

    if (traces.length === 0) {
        listEl.innerHTML = '<div class="trace-empty">Waiting for transactions... Perform actions (login, send message) to trigger traffic.</div>';
        return;
    }

    listEl.innerHTML = '';
    traces.forEach(trace => {
        const item = document.createElement('div');
        item.className = `trace-item status-${trace.status || 'success'}`;
        
        const timestamp = new Date(trace.timestamp).toLocaleTimeString();
        const latencyStr = trace.duration_ms !== undefined && trace.duration_ms > 0 ? `${trace.duration_ms}ms` : '';
        
        item.innerHTML = `
            <div class="trace-meta">
                <span class="trace-time">${timestamp}</span>
                <span class="trace-proto">${trace.protocol}</span>
                <span class="trace-type">${trace.type}</span>
                <span class="trace-latency">${latencyStr}</span>
            </div>
            <div class="trace-flow">
                <span class="trace-node src">${trace.source}</span>
                <span class="trace-arrow">➡️</span>
                <span class="trace-node dest">${trace.target}</span>
            </div>
            <div class="trace-msg">${trace.message}</div>
        `;
        listEl.appendChild(item);
    });
}

function flashNetworkLink(source, target) {
    let connectorId = null;
    let nodeSrc = null;
    let nodeDest = null;
    
    const src = source.toLowerCase();
    const dest = target.toLowerCase();
    
    if (src.includes('client') && dest.includes('gateway')) {
        connectorId = 'conn-client-gateway';
        nodeSrc = 'node-client';
        nodeDest = 'node-gateway';
    } else if (src.includes('gateway') && dest.includes('auth')) {
        connectorId = 'conn-gateway-services';
        nodeSrc = 'node-gateway';
        nodeDest = 'node-auth';
    } else if (src.includes('gateway') && dest.includes('client')) {
        connectorId = 'conn-client-gateway';
        nodeSrc = 'node-gateway';
        nodeDest = 'node-client';
    } else if (src.includes('gateway') && dest.includes('chat')) {
        connectorId = 'conn-gateway-services';
        nodeSrc = 'node-gateway';
        nodeDest = 'node-chat';
    } else if (src.includes('gateway') && dest.includes('call')) {
        connectorId = 'conn-gateway-services';
        nodeSrc = 'node-gateway';
        nodeDest = 'node-call';
    }
    
    if (connectorId) {
        const connEl = getElement(connectorId);
        if (connEl) {
            connEl.classList.add('flash-active');
            setTimeout(() => connEl.classList.remove('flash-active'), 800);
        }
    }
    if (nodeSrc) {
        const nodeEl = getElement(nodeSrc);
        if (nodeEl) {
            nodeEl.classList.add('pulse-active');
            setTimeout(() => nodeEl.classList.remove('pulse-active'), 800);
        }
    }
    if (nodeDest) {
        const nodeEl = getElement(nodeDest);
        if (nodeEl) {
            nodeEl.classList.add('pulse-active');
            setTimeout(() => nodeEl.classList.remove('pulse-active'), 800);
        }
    }
}
