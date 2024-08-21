// Toggles the side navigation menu
function toggleMenu() {
    const nav = document.getElementById("side-nav");
    nav.classList.toggle("open");

    const main = document.querySelector("main");
    main.classList.toggle("shifted");
}

document.getElementById("getSummaryNowBtn").addEventListener("click", function() {
    const summaryModalContent = document.getElementById("summaryModalContent");
    summaryModalContent.innerHTML = "Loading...";

    fetch("/profile/summary/now", {
        method: "GET",
        headers: {
            "Accept": "application/json",
        }
    })
    .then(response => response.json())
    .then(data => {
        if (data.message) {
            summaryModalContent.innerHTML = `<pre>${data.message}</pre>`;
        } else {
            summaryModalContent.innerHTML = "Failed to load summary.";
        }
    })
    .catch(error => {
        console.error("Error fetching summary:", error);
        summaryModalContent.innerHTML = "An error occurred while loading the summary.";
    });
});

// Handle incoming SSE events
const eventSource = new EventSource("/events");

eventSource.onmessage = function(event) {
    console.log("Received event:", event.data);

    let message;
    try {
        message = JSON.parse(event.data);
    } catch (e) {
        console.warn("Failed to parse JSON, treating as plain text:", event.data);
        message = event.data;
    }

    if (typeof message === "string") {
        const toastContainer = document.getElementById("toast-container");
        if (toastContainer) {
            handleToastNotification(message, toastContainer);
        }
    } else if (typeof message === "object" && message.event === "connection-status") {
        handleConnectionStatus(message.data);
    }
};

eventSource.onerror = function() {
    console.error("EventSource failed.");
};

// Function to handle toast notifications
function handleToastNotification(message, toastContainer) {
    let fullMessage = message;
    let severityClass = "info"; // Default severity class

    try {
        if (message.includes("(") && message.includes(")")) {
            const parts = message.split("(");
            fullMessage = parts[0].trim();
            severityClass = parts[1].replace(")", "").trim();
        }
    } catch (error) {
        console.error("Error processing event data:", error);
    }

    console.log("Creating toast with message:", fullMessage);

    const toast = document.createElement("div");
    toast.className = `toast align-items-center text-bg-${severityClass} border-0`;
    toast.role = "alert";
    toast.ariaLive = "assertive";
    toast.ariaAtomic = "true";

    toast.innerHTML = `
        <div class="d-flex">
            <div class="toast-body">
                ${fullMessage}
            </div>
            <button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast" aria-label="Close"></button>
        </div>
    `;

    toastContainer.appendChild(toast);

    const bootstrapToast = new bootstrap.Toast(toast);
    bootstrapToast.show();
}

// Function to handle connection status updates
function handleConnectionStatus(data) {
    const service = data.service;
    const status = data.status;
    const error = data.error || "";

    let iconElement;

    if (service === "zendesk") {
        iconElement = document.querySelector('.mdi-headset');
    } else if (service === "slack") {
        iconElement = document.querySelector('.mdi-slack');
    }

    if (iconElement) {
        const countSymbol = iconElement.nextElementSibling;
        if (countSymbol) {
            if (status === "connected") {
                countSymbol.className = "count-symbol bg-success";
                countSymbol.title = "Connected";
            } else if (status === "polling") {
                countSymbol.className = "count-symbol bg-warning";
                countSymbol.title = "Connecting...";
            } else if (status === "error") {
                countSymbol.className = "count-symbol bg-danger";
                countSymbol.title = error;
            }
        } else {
            console.warn("Could not find count-symbol element for", service);
        }
    } else {
        console.warn("Could not find icon element for service:", service);
    }
}
