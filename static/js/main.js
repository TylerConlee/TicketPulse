function toggleMenu() {
    const nav = document.getElementById("side-nav");
    nav.classList.toggle("open");

    const main = document.querySelector("main");
    main.classList.toggle("shifted");
}


const eventSource = new EventSource("/events");

        eventSource.onmessage = function(event) {
            console.log("Received event:", event.data); // Add this line

            const toastContainer = document.getElementById("toast-container");

            // Extract category, message, and severity from the event data
            const [fullMessage, severity] = event.data.split("(");
            const message = fullMessage.trim();
            const severityClass = severity.replace(")", "").trim();


            console.log("Creating toast with message:", message); // Debugging line

            // Create a new toast element
            const toast = document.createElement("div");
            toast.className = `toast align-items-center text-bg-${severityClass} border-0`;
            toast.role = "alert";
            toast.ariaLive = "assertive";
            toast.ariaAtomic = "true";

            toast.innerHTML = `
                <div class="d-flex">
                    <div class="toast-body">
                        ${message}
                    </div>
                    <button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast" aria-label="Close"></button>
                </div>
            `;

            // Append the toast to the container
            toastContainer.appendChild(toast);

            // Initialize the Bootstrap toast
            const bootstrapToast = new bootstrap.Toast(toast);
            bootstrapToast.show();
        };

        eventSource.onerror = function() {
            console.error("EventSource failed.");
        };


        document.getElementById("on-demand-summary-btn").addEventListener("click", function() {
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
                if (data.summary) {
                    summaryModalContent.innerHTML = data.summary;
                } else {
                    summaryModalContent.innerHTML = "Failed to load summary.";
                }
            })
            .catch(error => {
                console.error("Error fetching summary:", error);
                summaryModalContent.innerHTML = "An error occurred while loading the summary.";
            });
        });