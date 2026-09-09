import { useEffect } from "react";
import { useI18n } from "./I18nContext";
import { translateLegacyLiteral } from "./legacyTranslations";

const SKIP_TAGS = new Set(["SCRIPT", "STYLE", "CODE", "PRE", "TEXTAREA"]);
const ATTRIBUTES = ["placeholder", "title", "aria-label"] as const;

function translateTextNode(node: Text, language: "de" | "en") {
  const parent = node.parentElement;
  if (!parent || SKIP_TAGS.has(parent.tagName)) return;
  const raw = node.nodeValue ?? "";
  const trimmed = raw.trim();
  if (!trimmed) return;
  const translated = translateLegacyLiteral(trimmed, language);
  if (translated === trimmed) return;
  const start = raw.match(/^\s*/)?.[0] ?? "";
  const end = raw.match(/\s*$/)?.[0] ?? "";
  node.nodeValue = `${start}${translated}${end}`;
}

function translateElementAttributes(element: Element, language: "de" | "en") {
  if (!(element instanceof HTMLElement)) return;
  if (SKIP_TAGS.has(element.tagName)) return;
  for (const attr of ATTRIBUTES) {
    const raw = element.getAttribute(attr);
    if (!raw) continue;
    const translated = translateLegacyLiteral(raw.trim(), language);
    if (translated !== raw.trim()) element.setAttribute(attr, translated);
  }
}

function translateSubtree(root: Node, language: "de" | "en") {
  if (root.nodeType === Node.TEXT_NODE) {
    translateTextNode(root as Text, language);
    return;
  }
  if (root.nodeType !== Node.ELEMENT_NODE && root.nodeType !== Node.DOCUMENT_FRAGMENT_NODE) return;
  if (root.nodeType === Node.ELEMENT_NODE) translateElementAttributes(root as Element, language);
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT | NodeFilter.SHOW_TEXT);
  let current: Node | null = walker.nextNode();
  while (current) {
    if (current.nodeType === Node.TEXT_NODE) translateTextNode(current as Text, language);
    else if (current.nodeType === Node.ELEMENT_NODE) translateElementAttributes(current as Element, language);
    current = walker.nextNode();
  }
}

/**
 * Temporary compatibility layer for legacy components that still contain literal UI strings.
 * It translates exact known literals only, so user-entered content and video titles are untouched.
 */
export function LegacyI18nBridge() {
  const { language } = useI18n();

  useEffect(() => {
    const root = document.getElementById("root");
    if (!root) return;

    translateSubtree(root, language);
    const observer = new MutationObserver((mutations) => {
      for (const mutation of mutations) {
        if (mutation.type === "characterData") translateSubtree(mutation.target, language);
        for (const node of mutation.addedNodes) translateSubtree(node, language);
        if (mutation.type === "attributes") translateSubtree(mutation.target, language);
      }
    });
    observer.observe(root, { subtree: true, childList: true, characterData: true, attributes: true, attributeFilter: [...ATTRIBUTES] });
    return () => observer.disconnect();
  }, [language]);

  return null;
}
