import { store } from '../../state/store.js';
import { getElement } from './ui.js';
import { sendWSMessage, registerSocketListener } from './socket.js';

let localStream = null;
let peerConnection = null;
let currentCall = null; // { id, callerId, receiverId, callType, status }

const iceServers = {
    iceServers: [
        { urls: 'stun:stun.l.google.com:19302' },
        { urls: 'stun:stun1.l.google.com:19302' }
    ]
};

export async function initCall() {
    // 1. Fetch and append call.html to document body
    try {
        const res = await fetch('/components/call.html');
        if (res.ok) {
            const html = await res.text();
            const wrapper = document.createElement('div');
            wrapper.innerHTML = html;
            document.body.appendChild(wrapper.firstElementChild);
            setupCallListeners();
            registerCallSocketHandlers();
            console.log("Call UI module injected and initialized");
        } else {
            console.error("Failed to load call.html template");
        }
    } catch (err) {
        console.error("Error loading call components:", err);
    }
}

function setupCallListeners() {
    const acceptBtn = getElement('accept-call-btn');
    const rejectBtn = getElement('reject-call-btn');
    const startCallBtn = getElement('start-call-btn');

    if (acceptBtn) {
        acceptBtn.addEventListener('click', () => {
            acceptIncomingCall();
        });
    }

    if (rejectBtn) {
        rejectBtn.addEventListener('click', () => {
            declineOrEndCall();
        });
    }

    // Since the chat page can reload and button is rendered dynamically,
    // we use event delegation on document or poll when room changes
    document.addEventListener('click', (e) => {
        if (e.target && e.target.id === 'start-call-btn') {
            startOutgoingCall();
        }
    });
}

async function startOutgoingCall() {
    const activeRoom = store.activeRoom;
    if (!activeRoom || activeRoom.type !== 'direct') return;

    try {
        // Fetch members of the direct room to find the target user ID
        const token = localStorage.getItem('token');
        const res = await fetch(`/api/rooms/${activeRoom.id}/members`, {
            headers: { 'Authorization': `Bearer ${token}` }
        });

        if (!res.ok) {
            throw new Error(`Failed to load members: ${res.statusText}`);
        }

        const data = await res.json();
        const members = data.members || [];
        const peer = members.find(m => m.userId !== store.currentUser.id);
        if (!peer) {
            alert("No recipient found in this DM room.");
            return;
        }

        // Show Dialing UI
        showCallUI('dialing', peer.displayName || peer.username);

        // Send call.start event
        sendWSMessage('call.start', {
            roomId: activeRoom.id,
            targetUserId: peer.userId,
            callType: 'video'
        });

        currentCall = {
            callerId: store.currentUser.id,
            receiverId: peer.userId,
            callType: 'video',
            status: 'dialing'
        };

    } catch (err) {
        console.error("Error initiating call:", err);
        alert("Could not start call: " + err.message);
        hideCallUI();
    }
}

async function acceptIncomingCall() {
    if (!currentCall || !currentCall.id) return;

    try {
        showCallUI('connecting', 'Connecting...');
        
        // 1. Get User Media (Camera & Microphone)
        localStream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
        const localVideo = getElement('local-video');
        if (localVideo) {
            localVideo.srcObject = localStream;
        }

        // Show video stream UI
        const videoContainer = getElement('call-video-container');
        if (videoContainer) videoContainer.style.display = 'flex';
        const avatarsContainer = getElement('call-avatars-container');
        if (avatarsContainer) avatarsContainer.style.display = 'none';

        // 2. Initialize Peer Connection
        createPeerConnection();

        // 3. Send call.accept WS event
        sendWSMessage('call.accept', {
            callId: currentCall.id
        });

        currentCall.status = 'active';
        getElement('call-status-title').textContent = "Active Video Call";

    } catch (err) {
        console.error("Error accepting call:", err);
        alert("Camera/Mic access is required to connect video calls.");
        declineOrEndCall();
    }
}

function declineOrEndCall() {
    if (!currentCall) {
        hideCallUI();
        return;
    }

    if (currentCall.status === 'incoming') {
        sendWSMessage('call.reject', { callId: currentCall.id });
    } else if (currentCall.status === 'dialing' || currentCall.status === 'active') {
        if (currentCall.id) {
            sendWSMessage('call.end', { callId: currentCall.id });
        }
    }

    cleanupCall();
    hideCallUI();
}

function createPeerConnection() {
    if (peerConnection) return;

    peerConnection = new RTCPeerConnection(iceServers);

    // Track candidates to send
    peerConnection.onicecandidate = (event) => {
        if (event.candidate && currentCall && currentCall.id) {
            sendWSMessage('webrtc.ice-candidate', {
                callId: currentCall.id,
                candidate: event.candidate
            });
        }
    };

    // Receive remote video stream track
    peerConnection.ontrack = (event) => {
        const remoteVideo = getElement('remote-video');
        if (remoteVideo && event.streams && event.streams[0]) {
            remoteVideo.srcObject = event.streams[0];
        }
    };

    // Add local tracks to peer connection
    if (localStream) {
        localStream.getTracks().forEach(track => {
            peerConnection.addTrack(track, localStream);
        });
    }
}

function cleanupCall() {
    if (peerConnection) {
        peerConnection.close();
        peerConnection = null;
    }
    if (localStream) {
        localStream.getTracks().forEach(track => track.stop());
        localStream = null;
    }
    currentCall = null;
}

function showCallUI(status, displayName) {
    const overlay = getElement('call-overlay-view');
    if (!overlay) return;

    overlay.style.display = 'block';

    const statusTitle = getElement('call-status-title');
    const remoteUserLabel = getElement('remote-user-name');
    const acceptBtn = getElement('accept-call-btn');
    const rejectBtn = getElement('reject-call-btn');
    const videoContainer = getElement('call-video-container');
    const avatarsContainer = getElement('call-avatars-container');

    if (remoteUserLabel) remoteUserLabel.textContent = displayName;

    if (status === 'dialing') {
        statusTitle.textContent = "Dialing Call...";
        if (acceptBtn) acceptBtn.style.display = 'none';
        if (rejectBtn) rejectBtn.textContent = "Cancel";
        if (videoContainer) videoContainer.style.display = 'none';
        if (avatarsContainer) avatarsContainer.style.display = 'flex';
    } else if (status === 'incoming') {
        statusTitle.textContent = "Incoming Call...";
        if (acceptBtn) acceptBtn.style.display = 'block';
        if (rejectBtn) rejectBtn.textContent = "Decline";
        if (videoContainer) videoContainer.style.display = 'none';
        if (avatarsContainer) avatarsContainer.style.display = 'flex';
    } else if (status === 'connecting') {
        statusTitle.textContent = "Connecting Call...";
        if (acceptBtn) acceptBtn.style.display = 'none';
        if (rejectBtn) rejectBtn.textContent = "End Call";
    }
}

function hideCallUI() {
    const overlay = getElement('call-overlay-view');
    if (overlay) {
        overlay.style.display = 'none';
    }
    const videoContainer = getElement('call-video-container');
    if (videoContainer) videoContainer.style.display = 'none';
    const avatarsContainer = getElement('call-avatars-container');
    if (avatarsContainer) avatarsContainer.style.display = 'flex';
}

function registerCallSocketHandlers() {
    // 1. Incoming Call Event
    registerSocketListener('call.incoming', (data) => {
        currentCall = {
            id: data.callId,
            callerId: data.callerId,
            callType: data.callType,
            status: 'incoming'
        };

        const callerName = data.callerId === '11111111-1111-1111-1111-111111111111' ? 'Alice Henderson' : (data.callerId === '22222222-2222-2222-2222-222222222222' ? 'Bob Vance' : 'User');
        showCallUI('incoming', callerName);
    });

    // 2. Call Accepted Event (On Caller)
    registerSocketListener('call.accepted', async (data) => {
        if (!currentCall) return;
        currentCall.id = data.callId;
        currentCall.status = 'active';

        showCallUI('connecting', 'Connecting video...');
        getElement('call-status-title').textContent = "Active Video Call";

        try {
            // Get local camera & microphone streams
            localStream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
            const localVideo = getElement('local-video');
            if (localVideo) {
                localVideo.srcObject = localStream;
            }

            // Show video elements
            const videoContainer = getElement('call-video-container');
            if (videoContainer) videoContainer.style.display = 'flex';
            const avatarsContainer = getElement('call-avatars-container');
            if (avatarsContainer) avatarsContainer.style.display = 'none';

            // Create Peer Connection
            createPeerConnection();

            // Create Offer
            const offer = await peerConnection.createOffer();
            await peerConnection.setLocalDescription(offer);

            // Send Offer via WebSockets
            sendWSMessage('webrtc.offer', {
                callId: currentCall.id,
                sdp: offer
            });

        } catch (err) {
            console.error("Failed to setup WebRTC caller streams:", err);
            alert("Could not access camera/mic: " + err.message);
            declineOrEndCall();
        }
    });

    // 3. WebRTC Offer Event (On Receiver)
    registerSocketListener('webrtc.offer', async (data) => {
        if (!currentCall || !peerConnection) return;

        try {
            await peerConnection.setRemoteDescription(new RTCSessionDescription(data.sdp));
            const answer = await peerConnection.createAnswer();
            await peerConnection.setLocalDescription(answer);

            sendWSMessage('webrtc.answer', {
                callId: currentCall.id,
                sdp: answer
            });
        } catch (err) {
            console.error("Failed to handle WebRTC offer:", err);
        }
    });

    // 4. WebRTC Answer Event (On Caller)
    registerSocketListener('webrtc.answer', async (data) => {
        if (!currentCall || !peerConnection) return;
        try {
            await peerConnection.setRemoteDescription(new RTCSessionDescription(data.sdp));
        } catch (err) {
            console.error("Failed to set WebRTC answer:", err);
        }
    });

    // 5. ICE Candidate Event (On both sides)
    registerSocketListener('webrtc.ice-candidate', async (data) => {
        if (!peerConnection) return;
        try {
            if (data.candidate) {
                await peerConnection.addIceCandidate(new RTCIceCandidate(data.candidate));
            }
        } catch (err) {
            console.error("Failed to add WebRTC ICE candidate:", err);
        }
    });

    // 6. Call Rejected Event (On Caller)
    registerSocketListener('call.rejected', (data) => {
        alert("Call was declined.");
        cleanupCall();
        hideCallUI();
    });

    // 7. Call Ended Event (On both sides)
    registerSocketListener('call.ended', (data) => {
        cleanupCall();
        hideCallUI();
    });
}
