// Socket connection handling
import apiService from "./api-service.js";
import logRenderer from "./log-renderer.js";
import { logCounter, connectionStatus } from "./ui-components.js";

const socketHandler = {
    socket: null,
    isSearching: false,
    autoRefresh: true, // New flag to control auto-refresh

    init() {
        this.socket = io();
        this.setupSocketListeners();
    },

    setupSocketListeners() {
        this.socket.on("new_log", (log) => {
            if (!this.isSearching && this.autoRefresh) {  // Prevent updates if searching or auto-refresh is off
                try {
                    logRenderer.addNewLog(log);  // Append only the new log
                    logCounter.increment(); // Increment count instead of reloading everything
                    this.enforceLogLimit(); // Ensure log list doesn't get too long
                } catch (error) {
                    console.error("Error handling new log:", error);
                }
            }
        });

        this.socket.on("connect", () => {
            connectionStatus.setConnected(true);
        });

        this.socket.on("disconnect", () => {
            connectionStatus.setConnected(false);
        });
    },

    enforceLogLimit(maxLogs = 100) {
        const logContainer = document.getElementById("log-container");
        while (logContainer.children.length > maxLogs) {
            logContainer.removeChild(logContainer.lastChild);
        }
    },

    toggleAutoRefresh() {
        this.autoRefresh = !this.autoRefresh;
        console.log(`Auto Refresh: ${this.autoRefresh ? "ON" : "OFF"}`);
    }
};

export default socketHandler;