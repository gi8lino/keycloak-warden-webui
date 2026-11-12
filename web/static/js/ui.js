// A minimal enhancement for server-rendered navigation and form loading states.
// No API credentials or server-side session material are accessible to JavaScript.
(() => {
  const content = document.getElementById("page-content");
  const skeleton = document.getElementById("page-loading");
  if (!content || !skeleton) return;

  const reset = () => {
    content.hidden = false;
    skeleton.hidden = true;
    document.querySelectorAll('button[aria-busy="true"]').forEach((button) => {
      button.removeAttribute("aria-busy");
      button.disabled = false;
    });
  };

  const show = () => {
    content.hidden = true;
    skeleton.hidden = false;
  };

  window.addEventListener("pageshow", reset);
  document.addEventListener("submit", (event) => {
    const form = event.target;
    if (!(form instanceof HTMLFormElement) || !form.checkValidity()) return;
    const button = event.submitter;
    if (button instanceof HTMLButtonElement) {
      button.setAttribute("aria-busy", "true");
      button.disabled = true;
    }
    show();
  });
  document.addEventListener("click", (event) => {
    const link = event.target.closest?.("a[href]");
    if (!(link instanceof HTMLAnchorElement)) return;
    if (
      event.defaultPrevented ||
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.altKey ||
      event.shiftKey
    )
      return;
    if (
      link.origin !== location.origin ||
      link.target ||
      link.hasAttribute("download")
    )
      return;
    if (
      link.hash &&
      link.pathname === location.pathname &&
      link.search === location.search
    )
      return;
    show();
  });
})();
