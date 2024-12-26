// Socket connection handling
import apiService from './api-service.js';
import logRenderer from './log-renderer.js';
import {logCounter, connectionStatus } from './ui-components.js';

const socketHandler = {
    socket: null,
    isSearching: false,

    init() {
        this.socket = io();
        this.setupSocketListeners();
    },

    setupSocketListeners() {
        this.socket.on("new_log", async (log) => {
            const data = await apiService.fetchLogs();
            if (!this.isSearching) {
                try {
                    logRenderer.renderLogs(data.logs);
                    logCounter.setCount(data.logs.length);
                } catch (error) {
                    console.error("Error fetching logs:", error);
                }
            }
        });

        this.socket.on("connect", () => {
           connectionStatus.setConnected(true);
        });

        this.socket.on("disconnect", () => {
           connectionStatus.setConnected(false);
        });
    }
};

export default socketHandler;