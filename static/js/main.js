// SSE connection for real-time updates
const eventSource = new EventSource("/events");

eventSource.onmessage = function(event) {
  let message;
  try {
    message = JSON.parse(event.data);
  } catch (e) {
    message = event.data;
  }

  if (typeof message === "string") {
    showToast(message);
  } else if (typeof message === "object" && message.event === "connection-status") {
    handleConnectionStatus(message.data);
  }
};

eventSource.onerror = function() {
  console.error("SSE connection lost. Reconnecting...");
};

function showToast(message) {
  const container = document.getElementById("toast-container");
  if (!container) return;

  let text = message;
  let type = "info";

  try {
    if (message.includes("(") && message.includes(")")) {
      const parts = message.split("(");
      text = parts[0].trim();
      type = parts[1].replace(")", "").trim();
    }
  } catch (e) {}

  const colors = {
    info: "bg-blue-600",
    success: "bg-green-600",
    warning: "bg-amber-500",
    danger: "bg-red-600",
    error: "bg-red-600"
  };

  const toast = document.createElement("div");
  toast.className = (colors[type] || colors.info) + " text-white px-4 py-3 rounded-lg shadow-lg text-sm flex items-center gap-3 animate-slide-in";

  const span = document.createElement("span");
  span.className = "flex-1";
  span.textContent = text;
  toast.appendChild(span);

  const btn = document.createElement("button");
  btn.className = "opacity-70 hover:opacity-100 transition-opacity";
  btn.onclick = function() { toast.remove(); };
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("class", "w-4 h-4");
  svg.setAttribute("fill", "none");
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("stroke", "currentColor");
  svg.setAttribute("stroke-width", "2");
  const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
  path.setAttribute("stroke-linecap", "round");
  path.setAttribute("stroke-linejoin", "round");
  path.setAttribute("d", "M6 18L18 6M6 6l12 12");
  svg.appendChild(path);
  btn.appendChild(svg);
  toast.appendChild(btn);

  container.appendChild(toast);

  setTimeout(function() {
    toast.style.opacity = "0";
    toast.style.transform = "translateX(100%)";
    toast.style.transition = "opacity 0.3s, transform 0.3s";
    setTimeout(function() { toast.remove(); }, 300);
  }, 5000);
}

function handleConnectionStatus(data) {
  const statusMap = {
    connected: { color: "bg-green-400", title: "Connected" },
    polling: { color: "bg-yellow-400", title: "Connecting..." },
    error: { color: "bg-red-400", title: data.error || "Error" }
  };

  const status = statusMap[data.status] || statusMap.polling;
  const elementId = data.service === "zendesk" ? "zendesk-status" : "slack-status";
  const el = document.getElementById(elementId);

  if (el) {
    el.className = "w-2 h-2 rounded-full " + status.color;
    el.title = status.title;
  }
}

// Tag autocomplete for alerts pages
(function() {
  var cachedTags = null;
  var activeDropdown = null;

  function fetchTags(callback) {
    if (cachedTags !== null) {
      callback(cachedTags);
      return;
    }
    fetch("/alerts/tags")
      .then(function(r) { return r.json(); })
      .then(function(tags) {
        cachedTags = tags || [];
        callback(cachedTags);
      })
      .catch(function() {
        cachedTags = [];
        callback(cachedTags);
      });
  }

  function filterTags(tags, query) {
    if (!query) return tags.slice(0, 20);
    var q = query.toLowerCase();
    return tags.filter(function(t) { return t.toLowerCase().indexOf(q) !== -1; }).slice(0, 20);
  }

  function showDropdown(input, dropdown, tags) {
    if (tags.length === 0) {
      dropdown.classList.add("hidden");
      return;
    }
    dropdown.innerHTML = "";
    tags.forEach(function(tag) {
      var item = document.createElement("div");
      item.className = "px-3 py-2 text-sm cursor-pointer hover:bg-brand-50 hover:text-brand-700 transition-colors";
      item.textContent = tag;
      item.addEventListener("mousedown", function(e) {
        e.preventDefault();
        input.value = tag;
        dropdown.classList.add("hidden");
      });
      dropdown.appendChild(item);
    });
    dropdown.classList.remove("hidden");
    activeDropdown = dropdown;
  }

  function initAutocomplete() {
    var inputs = document.querySelectorAll(".tag-autocomplete-input");
    if (inputs.length === 0) return;

    inputs.forEach(function(input) {
      var dropdown = input.parentElement.querySelector(".tag-autocomplete-dropdown");
      if (!dropdown) return;

      input.addEventListener("focus", function() {
        fetchTags(function(tags) {
          showDropdown(input, dropdown, filterTags(tags, input.value));
        });
      });

      input.addEventListener("input", function() {
        fetchTags(function(tags) {
          showDropdown(input, dropdown, filterTags(tags, input.value));
        });
      });

      input.addEventListener("blur", function() {
        setTimeout(function() { dropdown.classList.add("hidden"); }, 150);
      });

      input.addEventListener("keydown", function(e) {
        if (e.key === "Escape") {
          dropdown.classList.add("hidden");
        }
      });
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initAutocomplete);
  } else {
    initAutocomplete();
  }
})();
