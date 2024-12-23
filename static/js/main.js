import socketHandler from './socket-handler.js';
import logRenderer from './log-renderer.js';
import apiService from './api-service.js';
import { logCounter } from './ui-components.js';

// Initialize search functionality
const searchInput = document.getElementById("search-input");
const searchButton = document.getElementById("search-button");
const clearButton = document.getElementById("clear-search");

async function handleSearch() {
    const query = searchInput.value.trim();
    socketHandler.isSearching = !!query;

    try {
        const data = await apiService.fetchLogs(query);
        logRenderer.renderLogs(data.logs);
        logCounter.setCount(data.logs.length);
    } catch (error) {
        // Handle error (could add a UI notification here)
    }
}

// Event listeners
searchButton.addEventListener("click", handleSearch);
clearButton.addEventListener("click", () => {
    searchInput.value = "";
    handleSearch();
});

// Quick filters
document.querySelectorAll('.filter-badge').forEach(badge => {
    badge.addEventListener('click', () => {
        searchInput.value = badge.dataset.filter;
        handleSearch();
    });
});

// Trigger search on Enter key press
searchInput.addEventListener('keydown', (event) => {
    if (event.key === 'Enter') {
        handleSearch();
    }
});


// Initialize
socketHandler.init();
handleSearch(); // Initial load