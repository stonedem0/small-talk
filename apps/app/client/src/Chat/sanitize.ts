import DOMPurify from "dompurify";

// Chat messages are rendered as HTML (bold, links, ...), so every message is
// run through DOMPurify with a small allowlist. DOMPurify parses in an inert
// document; sanitizing a parsed copy by hand is not safe, because assigning
// innerHTML starts loading <img>/<svg> content and fires their handlers
// (onerror, onload) before any cleanup can run.
const ALLOWED_TAGS = ["strong", "em", "u", "del", "code", "a"];
const ALLOWED_ATTR = ["href", "target", "rel"];

DOMPurify.addHook("afterSanitizeAttributes", (node) => {
  if (node.tagName === "A") {
    const href = (node.getAttribute("href") ?? "").trim();
    if (!/^https?:\/\//i.test(href)) node.removeAttribute("href");
    node.setAttribute("target", "_blank");
    node.setAttribute("rel", "noopener noreferrer");
    node.classList.add("msg-link");
  }
  if (node.tagName === "CODE") node.classList.add("msg-code");
});

export const sanitizeHtml = (dirty: string): string =>
  DOMPurify.sanitize(dirty, { ALLOWED_TAGS, ALLOWED_ATTR });
