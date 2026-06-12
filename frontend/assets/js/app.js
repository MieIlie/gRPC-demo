import { checkAuth } from './auth.js';
import { initCall } from './call.js';

document.addEventListener('DOMContentLoaded', () => {
    console.log("App bootstrapping...");
    initCall();
    checkAuth();
});
