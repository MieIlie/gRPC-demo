import { store } from '../../state/store.js';

let eventSource = null;

export function initTraceListener() {
    if (eventSource) {
        eventSource.close();
    }

    // Connect to Server-Sent Events endpoint on the gateway
    eventSource = new EventSource(`/api/traces`);

    eventSource.onmessage = (event) => {
        try {
            const traceEvent = JSON.parse(event.data);
            const currentTraces = [...store.traces];
            currentTraces.unshift(traceEvent); // Place newest trace at the top
            
            // Limit in-memory history to 100 entries to prevent memory leak
            if (currentTraces.length > 100) {
                currentTraces.pop();
            }
            
            store.setState({ traces: currentTraces });
        } catch (err) {
            console.error("Failed to parse trace event:", err);
        }
    };

    eventSource.onerror = (err) => {
        console.error("EventSource trace stream connection error:", err);
    };
}

export function clearTraces() {
    store.setState({ traces: [] });
}
