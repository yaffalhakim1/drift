/* drift — theme switch.
   The page boots with the right theme from an inline script; this module only
   handles changing it, remembering it, and keeping the browser chrome in sync. */

window.initTheme = function initTheme(onChange) {
  const root = document.documentElement;
  const button = document.getElementById("theme");
  const meta = document.querySelector('meta[name="theme-color"]');
  const system = window.matchMedia("(prefers-color-scheme: light)");
  const CHROME = { dark: "#08080a", light: "#f1ece3" };

  const stored = () => {
    try { return localStorage.getItem("drift-theme"); } catch (_) { return null; }
  };

  function paint(theme) {
    root.setAttribute("data-theme", theme);
    const light = theme === "light";
    button.setAttribute("aria-pressed", String(light));
    button.setAttribute("aria-label", light ? "Switch to dark theme" : "Switch to light theme");
    if (meta) meta.setAttribute("content", CHROME[theme]);
    if (onChange) onChange(theme);
  }

  function apply(theme, remember) {
    paint(theme);
    if (!remember) return;
    try { localStorage.setItem("drift-theme", theme); } catch (_) { /* storage blocked */ }
  }

  paint(root.getAttribute("data-theme") === "light" ? "light" : "dark");

  button.addEventListener("click", () => {
    const next = root.getAttribute("data-theme") === "light" ? "dark" : "light";
    apply(next, true);
  });

  /* Follow the system only while the user has not chosen a side. */
  system.addEventListener("change", (event) => {
    if (stored()) return;
    apply(event.matches ? "light" : "dark", false);
  });
};
