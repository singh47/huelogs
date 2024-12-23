// API calls handling
const apiService = {
    async fetchLogs(query = "") {
        try {
            const url = query 
                ? `/api/search-logs?q=${encodeURIComponent(query)}`
                : "/api/logs";
            
            const response = await fetch(url);
            
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            return await response.json();
        } catch (error) {
            console.error("Error fetching logs:", error);
            throw error;
        }
    }
};

export default apiService;