import { store } from '../../state/store.js';
import { getElement } from './ui.js';
import { logout } from './auth.js';
import { sendWSMessage, registerSocketListener } from './socket.js';

export function initChat() {
    setupChatDOM();
    setupChatListeners();
    registerSocketHandlers();
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

    logoutBtn.addEventListener('click', () => {
        logout();
    });

    createRoomBtn.addEventListener('click', () => {
        const name = prompt("Enter room name:");
        if (name) {
            const newRoom = { id: 'room-placeholder-id-' + Date.now(), name, type: 'group' };
            store.setState({ rooms: [...store.rooms, newRoom] });
            renderRooms();
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

    store.subscribe((state) => {
        if (state.activeRoom) {
            input.disabled = false;
            sendBtn.disabled = false;
            getElement('active-room-title').textContent = state.activeRoom.name;
        } else {
            input.disabled = true;
            sendBtn.disabled = true;
            getElement('active-room-title').textContent = "Select or Create a Room";
        }
    });

    renderRooms();
}

function renderRooms() {
    const listEl = getElement('rooms-list');
    if (!listEl) return;
    listEl.innerHTML = '';

    const defaultRoom = { id: 'a3333333-3333-3333-3333-333333333333', name: 'General Room', type: 'group' };
    const allRooms = [defaultRoom, ...store.rooms];

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
            const container = getElement('messages-container');
            if (container) {
                container.innerHTML = `
                    <div style="text-align: center; color: var(--text-muted); margin-top: 2rem;">
                        Connected to ${room.name}. Send a message!
                    </div>
                `;
            }
        });
        listEl.appendChild(item);
    });
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
        
        bubble.innerHTML = `
            <div>${data.content}</div>
            <div class="message-meta">
                <span>${isOutgoing ? 'You' : (data.senderName || 'User')}</span>
                <span>${data.createdAt || 'now'}</span>
            </div>
        `;

        container.appendChild(bubble);
        container.scrollTop = container.scrollHeight;
    });
}
