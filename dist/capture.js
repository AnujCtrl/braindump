// Tauri 2 with `withGlobalTauri: true` exposes window.__TAURI__.
const { invoke } = window.__TAURI__.core;
const { getCurrentWindow } = window.__TAURI__.window;

const textarea = document.getElementById("capture");
const statusEl = document.getElementById("status");
const closeBtn = document.getElementById("close");
const w = getCurrentWindow();

async function submit() {
  const text = textarea.value.trim();
  if (!text) return;
  try {
    const result = await invoke("submit_capture", { input: text });
    statusEl.classList.remove("error");
    statusEl.classList.add("ok");
    statusEl.textContent = `captured ${result.count} todo${result.count === 1 ? "" : "s"}`;
    textarea.value = "";
    // Soft tick (~80ms) lets the user register the confirmation, then dismiss.
    setTimeout(() => w.hide(), 80);
  } catch (err) {
    statusEl.classList.remove("ok");
    statusEl.classList.add("error");
    statusEl.textContent = String(err);
  }
}

async function dismiss() {
  textarea.value = "";
  statusEl.classList.remove("ok", "error");
  statusEl.textContent = "";
  await w.hide();
}

textarea.addEventListener("keydown", (event) => {
  // Ctrl/Cmd+Enter submits; bare Enter inserts a newline (multi-line capture
  // and dump mode both rely on that).
  if ((event.ctrlKey || event.metaKey) && event.key === "Enter") {
    event.preventDefault();
    submit();
    return;
  }
  if (event.key === "Escape") {
    event.preventDefault();
    dismiss();
    return;
  }
  // Ctrl/Cmd+X / Ctrl/Cmd+W also dismiss — the user explicitly asked for
  // an X-key affordance and ⌘W is a Mac convention. Bare X is a typed
  // character; we don't intercept that.
  if ((event.ctrlKey || event.metaKey) && (event.key === "x" || event.key === "X" || event.key === "w" || event.key === "W")) {
    event.preventDefault();
    dismiss();
  }
});

closeBtn.addEventListener("click", () => dismiss());

// Re-focus the textarea every time the window is shown.
w.onFocusChanged(({ payload: focused }) => {
  if (focused) {
    textarea.focus();
    textarea.select();
  }
});
