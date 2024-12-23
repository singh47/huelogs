// Socket connection handling
const socketHandler = {
    socket: null,
    isSearching: false,

    init() {
        this.socket = io();
        this.setupSocketListeners();
    },

    setupSocketListeners() {
        this.socket.on("new_log", (log) => {
            if (!this.isSearching) {
                logRenderer.addNewLog(log);
                logCounter.increment();
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