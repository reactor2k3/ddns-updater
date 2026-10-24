try {
  const previewTheme = new URLSearchParams(location.search).get("theme");
  document.documentElement.dataset.theme = ["light", "dark"].includes(
    previewTheme,
  )
    ? previewTheme
    : localStorage.getItem("ddns-theme") ||
      (matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
} catch {}
