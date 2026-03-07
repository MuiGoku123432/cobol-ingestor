// Auth state
let authPollingInterval = null;

async function checkAuth() {
  try {
    const resp = await fetch("/api/auth/status");
    const data = await resp.json();
    if (data.authenticated) {
      hideAuthOverlay();
      if (data.provider === "copilot") {
        document.getElementById("logoutBtn").classList.remove("hidden");
      }
    } else if (data.provider === "copilot") {
      showAuthOverlay();
      if (data.pending) {
        // Already polling, just show pending state and start status polling
        document.getElementById("authLogin").classList.add("hidden");
        document.getElementById("authPending").classList.remove("hidden");
        startAuthPolling();
      }
    }
  } catch {
    // Server not ready, retry
    setTimeout(checkAuth, 2000);
  }
}

function showAuthOverlay() {
  document.getElementById("authOverlay").classList.remove("hidden");
}

function hideAuthOverlay() {
  document.getElementById("authOverlay").classList.add("hidden");
  stopAuthPolling();
  loadModels();
}

async function startLogin() {
  const loginBtn = document.getElementById("authLogin");
  const pendingEl = document.getElementById("authPending");
  const errorEl = document.getElementById("authError");

  errorEl.classList.add("hidden");

  try {
    const resp = await fetch("/api/auth/device-code", { method: "POST" });
    const data = await resp.json();

    if (data.authenticated) {
      hideAuthOverlay();
      return;
    }

    if (data.error) {
      errorEl.textContent = data.error;
      errorEl.classList.remove("hidden");
      return;
    }

    // Show code and link
    document.getElementById("authCode").textContent = data.user_code;
    const link = document.getElementById("authLink");
    link.href = data.verification_uri;

    loginBtn.classList.add("hidden");
    pendingEl.classList.remove("hidden");

    startAuthPolling();
  } catch (err) {
    errorEl.textContent = "Failed to start login: " + err.message;
    errorEl.classList.remove("hidden");
  }
}

function startAuthPolling() {
  stopAuthPolling();
  authPollingInterval = setInterval(async () => {
    try {
      const resp = await fetch("/api/auth/status");
      const data = await resp.json();
      if (data.authenticated) {
        hideAuthOverlay();
        if (data.provider === "copilot") {
          document.getElementById("logoutBtn").classList.remove("hidden");
        }
      }
    } catch {
      // ignore
    }
  }, 2000);
}

function stopAuthPolling() {
  if (authPollingInterval) {
    clearInterval(authPollingInterval);
    authPollingInterval = null;
  }
}

async function logout() {
  try {
    await fetch("/api/auth/logout", { method: "POST" });
    document.getElementById("logoutBtn").classList.add("hidden");
    // Reset auth overlay state
    document.getElementById("authLogin").classList.remove("hidden");
    document.getElementById("authPending").classList.add("hidden");
    document.getElementById("authError").classList.add("hidden");
    showAuthOverlay();
  } catch {
    // ignore
  }
}

// Model picker
async function loadModels() {
  const container = document.getElementById("modelSelectContainer");
  const select = document.getElementById("modelSelect");

  try {
    const resp = await fetch("/api/models");
    if (!resp.ok) return;

    const models = await resp.json();
    if (!models || models.length === 0) return;

    select.innerHTML = models
      .map((m) => `<option value="${m.id}">${m.name || m.id}</option>`)
      .join("");
    select.disabled = false;
    container.classList.remove("hidden");

    // Auto-select first model
    selectModel(models[0].id);
  } catch {
    // ignore — models not available
  }
}

async function selectModel(modelId) {
  try {
    await fetch("/api/models/select", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ model: modelId }),
    });
  } catch {
    // ignore
  }
}

document.getElementById("modelSelect").addEventListener("change", (e) => {
  selectModel(e.target.value);
});

// Check auth on load
checkAuth();

// Client-side message history sent with each request
let messages = [];
let isStreaming = false;

// Framework options per language
const frameworks = {
  Java: ["Spring Boot", "Quarkus", "Micronaut", "None"],
  "C#": [".NET 8", "ASP.NET Core", "None"],
  Python: ["FastAPI", "Django", "Flask", "None"],
  Go: ["Gin", "Echo", "Chi", "None"],
  TypeScript: ["NestJS", "Express", "Fastify", "None"],
  Kotlin: ["Spring Boot", "Ktor", "None"],
};

// Initialize framework dropdown
const langSelect = document.getElementById("targetLanguage");
const fwSelect = document.getElementById("framework");

function updateFrameworks() {
  const lang = langSelect.value;
  const opts = frameworks[lang] || ["None"];
  fwSelect.innerHTML = opts
    .map((f) => `<option value="${f === "None" ? "" : f}">${f}</option>`)
    .join("");
}
langSelect.addEventListener("change", updateFrameworks);
updateFrameworks();

// Configure marked
marked.setOptions({
  highlight: function (code, lang) {
    if (lang && hljs.getLanguage(lang)) {
      return hljs.highlight(code, { language: lang }).value;
    }
    return hljs.highlightAuto(code).value;
  },
  breaks: true,
});

function addMessage(role, html) {
  const container = document.getElementById("chatMessages");

  // Remove placeholder if present
  const placeholder = container.querySelector(".text-center");
  if (placeholder) placeholder.remove();

  const wrapper = document.createElement("div");
  wrapper.className = `flex ${role === "user" ? "justify-end" : "justify-start"}`;

  const bubble = document.createElement("div");
  bubble.className =
    role === "user"
      ? "max-w-2xl bg-indigo-600 rounded-2xl rounded-br-md px-4 py-3 text-sm"
      : "max-w-4xl bg-gray-800 rounded-2xl rounded-bl-md px-4 py-3 text-sm message-content";
  bubble.innerHTML = html;

  wrapper.appendChild(bubble);
  container.appendChild(wrapper);
  container.scrollTop = container.scrollHeight;
  return bubble;
}

function addToolIndicator(name, id) {
  const container = document.getElementById("chatMessages");
  const indicator = document.createElement("div");
  indicator.id = `tool-${id}`;
  indicator.className =
    "tool-indicator flex items-center gap-2 text-xs text-gray-400 ml-2 my-1";
  indicator.innerHTML = `
    <span class="pulse-dot inline-block w-2 h-2 rounded-full bg-amber-400"></span>
    <span>Calling <code class="bg-gray-800 px-1.5 py-0.5 rounded text-amber-300">${name}</code>...</span>
  `;
  container.appendChild(indicator);
  container.scrollTop = container.scrollHeight;
}

function updateToolIndicator(id, result, isError) {
  const indicator = document.getElementById(`tool-${id}`);
  if (!indicator) return;
  const dot = indicator.querySelector(".pulse-dot");
  if (dot) {
    dot.classList.remove("pulse-dot", "bg-amber-400");
    dot.classList.add(isError ? "bg-red-400" : "bg-green-400");
  }
  const span = indicator.querySelector("span:last-child");
  if (span && result) {
    const preview =
      result.length > 100 ? result.substring(0, 100) + "..." : result;
    span.innerHTML += ` <span class="text-gray-500">→ ${escapeHtml(preview)}</span>`;
  }
}

function escapeHtml(text) {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}

function setLoading(loading) {
  isStreaming = loading;
  const btn = document.getElementById("sendBtn");
  const input = document.getElementById("chatInput");
  btn.disabled = loading;
  input.disabled = loading;
  btn.textContent = loading ? "..." : "Send";
}

async function sendMessage(e) {
  e.preventDefault();
  const input = document.getElementById("chatInput");
  const text = input.value.trim();
  if (!text || isStreaming) return;

  // Add user message
  messages.push({ role: "user", content: text });
  addMessage("user", escapeHtml(text));
  input.value = "";
  setLoading(true);

  let assistantBubble = null;
  let assistantText = "";

  try {
    const response = await fetch("/api/chat", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        messages: messages,
        targetLanguage: langSelect.value,
        framework: fwSelect.value,
      }),
    });

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split("\n");
      buffer = lines.pop(); // keep incomplete line

      let eventType = "";
      for (const line of lines) {
        if (line.startsWith("event: ")) {
          eventType = line.substring(7).trim();
        } else if (line.startsWith("data: ")) {
          const data = line.substring(6);
          handleSSEEvent(eventType, data);
        }
      }
    }

    function handleSSEEvent(event, dataStr) {
      let data;
      try {
        data = JSON.parse(dataStr);
      } catch {
        data = dataStr;
      }

      switch (event) {
        case "text":
          assistantText += data.content || "";
          const rendered = marked.parse(assistantText);
          if (!assistantBubble) {
            assistantBubble = addMessage("assistant", rendered);
          } else {
            assistantBubble.innerHTML = rendered;
          }
          // Re-highlight code blocks
          assistantBubble.querySelectorAll("pre code").forEach((block) => {
            if (!block.dataset.highlighted) {
              hljs.highlightElement(block);
              block.dataset.highlighted = "true";
            }
          });
          break;

        case "tool_start":
          addToolIndicator(data.name, data.id);
          break;

        case "tool_result":
          updateToolIndicator(
            data.id,
            data.result || data.error,
            !!data.error
          );
          break;

        case "done":
          if (assistantText) {
            messages.push({ role: "assistant", content: assistantText });
          }
          break;

        case "error":
          const errMsg = data.error || "Unknown error";
          addMessage(
            "assistant",
            `<span class="text-red-400">Error: ${escapeHtml(errMsg)}</span>`
          );
          break;
      }
    }
  } catch (err) {
    addMessage(
      "assistant",
      `<span class="text-red-400">Connection error: ${escapeHtml(err.message)}</span>`
    );
  } finally {
    setLoading(false);
  }
}

function clearChat() {
  messages = [];
  const container = document.getElementById("chatMessages");
  container.innerHTML = `
    <div class="text-center text-gray-500 mt-20">
      <p class="text-lg">Ask about your COBOL programs</p>
      <p class="text-sm mt-1">I'll use the graph database to understand and translate them</p>
    </div>
  `;
}
