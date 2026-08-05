mermaid.initialize({
  startOnLoad: false,
  theme: window.matchMedia("(prefers-color-scheme: dark)").matches
    ? "dark"
    : "default",
  securityLevel: "loose",
});

document.addEventListener("DOMContentLoaded", function () {
  mermaid.run({ querySelector: ".mermaid" });
});
