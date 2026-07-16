import { checkAuth } from './auth.js';
import { startConnectionPolling } from './connection.js';
import { initTraceListener } from './traces.js';

document.addEventListener('DOMContentLoaded', () => {
    console.log("App bootstrapping...");
    startConnectionPolling();
    initTraceListener();
    checkAuth();
});
