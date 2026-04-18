// WebSocket connection — replaces Socket.IO with the browser's native API.
// The Go server sends raw JSON objects over a plain WebSocket; no Socket.IO
// framing or handshake is needed.
import logRenderer from './log-renderer.js';
import { logCounter, connectionStatus } from './ui-components.js';

const socketHandler = {
    socket: null,
    isSearching: false,
    autoRefresh: true,
    _reconnectDelay: 1000, // ms, doubles on each failed attempt up to 30 s

    init() {
        this._connect();
    },

    _connect() {
        const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        this.socket = new WebSocket(`${proto}//${window.location.host}/ws`);

        this.socket.onopen = () => {
            connectionStatus.setConnected(true);
            this._reconnectDelay = 1000; // reset backoff on successful connect
        };

        this.socket.onmessage = (event) => {
            if (!this.isSearching && this.autoRefresh) {
                try {
                    const log = JSON.parse(event.data);
                    logRenderer.addNewLog(log);
                    logCounter.increment();
                    this._enforceLogLimit();
                } catch (err) {
                    console.error('Failed to parse WebSocket message:', err);
                }
            }
        };

        this.socket.onclose = () => {
            connectionStatus.setConnected(false);
            // Exponential backoff — reconnect without hammering the server.
            setTimeout(() => this._connect(), this._reconnectDelay);
            this._reconnectDelay = Math.min(this._reconnectDelay * 2, 30_000);
        };

        this.socket.onerror = () => {
            // onclose fires immediately after onerror, so reconnect is handled there.
            connectionStatus.setConnected(false);
        };
    },

    _enforceLogLimit(maxLogs = 100) {
        const container = document.getElementById('log-container');
        while (container.children.length > maxLogs) {
            container.removeChild(container.lastChild);
        }
    },

    toggleAutoRefresh() {
        this.autoRefresh = !this.autoRefresh;
    },
};

export default socketHandler;
