(function () {
  try {
    var t = localStorage.getItem("theme") || "system";
    var resolved =
      t === "system"
        ? matchMedia("(prefers-color-scheme: dark)").matches
          ? "dark"
          : "light"
        : t;
    document.documentElement.setAttribute("data-theme", resolved);
  } catch (_) {}
})();
