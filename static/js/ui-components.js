// UI components handling
export const connectionStatus = {
    element: document.getElementById("connection-status"),
    indicator: document.querySelector(".bi-circle-fill"),

    setConnected(isConnected) {
        this.element.textContent = isConnected ? "Connected" : "Disconnected";
        this.indicator.classList.toggle("text-success", isConnected);
        this.indicator.classList.toggle("text-danger", !isConnected);
    }
};

export const logCounter = {
    element: document.getElementById("log-count"),
    count: 0,

    increment() {
        this.count++;
        this.update();
    },

    setCount(count) {
        this.count = count;
        this.update();
    },

    update() {
        this.element.textContent = this.count;
    }
};