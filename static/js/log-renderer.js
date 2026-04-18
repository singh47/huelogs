// Log rendering functionality
const LEVEL_CLASSES = {
    ERROR:    'log-level-error',
    CRITICAL: 'log-level-error',
    WARNING:  'log-level-warning',
    DEBUG:    'log-level-debug',
    INFO:     'log-level-info',
};

const logRenderer = {
    container: document.getElementById("log-container"),

    createLogEntry(log) {
        const level = (log.level || 'INFO').toUpperCase();

        const logEntry = document.createElement("div");
        logEntry.className = `log-entry ${LEVEL_CLASSES[level] || ''}`;
        logEntry.dataset.service = log.service_name || '';
        logEntry.dataset.level = level;

        // Level badge
        const levelBadge = document.createElement("span");
        levelBadge.className = `log-level-badge log-level-badge-${level.toLowerCase()}`;
        levelBadge.textContent = level;
        logEntry.appendChild(levelBadge);

        // Service name
        if (log.service_name) {
            const service = document.createElement("span");
            service.className = "log-service";
            service.textContent = `[${log.service_name}]`;
            logEntry.appendChild(service);
        }

        // Timestamp
        const timestamp = document.createElement("span");
        timestamp.className = "log-timestamp";
        timestamp.textContent = `[${log.timestamp}]`;
        logEntry.appendChild(timestamp);

        // Message
        const message = document.createElement("span");
        message.className = "log-message ms-2";
        message.textContent = log.message;
        logEntry.appendChild(message);

        return logEntry;
    },

    addNewLog(log) {
        const logEntry = this.createLogEntry(log);
        this.container.prepend(logEntry);
    },

    clearLogs() {
        this.container.innerHTML = "";
    },

    renderLogs(logs) {
        this.clearLogs();
        logs.forEach(log => {
            const logEntry = this.createLogEntry(log);
            this.container.appendChild(logEntry);
        });
    }
};

export default logRenderer;
