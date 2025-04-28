// Log rendering functionality
const logRenderer = {
    container: document.getElementById("log-container"),

    createLogEntry(log) {
        const logEntry = document.createElement("div");
        logEntry.className = "log-entry";
        logEntry.dataset.service = log.service_name || '';

        // Service name (if present)
        if (log.service_name) {
            const service = document.createElement("span");
            service.className = "log-service";
            service.textContent = `[${log.service_name}]`;
            logEntry.appendChild(service);
        }
        
        const timestamp = document.createElement("span");
        timestamp.className = "log-timestamp";
        timestamp.textContent = `[${log.timestamp}]`;
        logEntry.appendChild(timestamp);

        const message = document.createElement("span");
        message.className = "log-message ms-2";
        message.textContent = log.message;
        logEntry.appendChild(message);
        
        // Add appropriate class based on log level
        if (log.message.toLowerCase().includes("error")) {
            logEntry.classList.add("text-danger");
        } else if (log.message.toLowerCase().includes("warning")) {
            logEntry.classList.add("text-warning");
        }

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