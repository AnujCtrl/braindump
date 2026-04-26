// Tauri 2 with `withGlobalTauri: true` exposes window.__TAURI__.
const { invoke } = window.__TAURI__.core;
const { getCurrentWindow } = window.__TAURI__.window;

const textarea = document.getElementById("capture");
const status = document.getElementById("status");
const w = getCurrentWindow();

async function submit() {
  const text = textarea.value.trim();
  if (!text) return;
  try {
    const result = await invoke("submit_capture", { input: text });
    status.classList.remove("error");
    status.textContent = `captured ${result.count} todo${result.count === 1 ? "" : "s"}`;
    textarea.value = "";
    // Soft tick (~80ms) lets the user register the confirmation, then dismiss.
    setTimeout(() => w.hide(), 80);
  } catch (err) {
    status.classList.add("error");
    status.textContent = String(err);
  }
}

async function dismiss() {
  textarea.value = "";
  status.textContent = "";
  await w.hide();
}

textarea.addEventListener("keydown", (event) => {
  // Ctrl/Cmd+Enter submits; bare Enter inserts a newline (multi-line capture
  // and dump mode both rely on that).
  if ((event.ctrlKey || event.metaKey) && event.key === "Enter") {
    event.preventDefault();
    submit();
  } else if (event.key === "Escape") {
    event.preventDefault();
    dismiss();
  }
});

// Re-focus the textarea every time the window is shown.
w.onFocusChanged(({ payload: focused }) => {
  if (focused) {
    textarea.focus();
    textarea.select();
  }
});
