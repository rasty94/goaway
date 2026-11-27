import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import i18n from "@/i18n";
import { afterEach, vi } from "vitest";

await i18n.changeLanguage("en");

// Browser APIs jsdom does not implement but Radix/recharts/hooks use.
globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};
globalThis.IntersectionObserver ??= class {
  root = null;
  rootMargin = "";
  thresholds = [];
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords() {
    return [];
  }
} as unknown as typeof IntersectionObserver;
// jsdom has no 2D canvas; a context whose every method is a no-op keeps drawing code running.
const noop2d: CanvasRenderingContext2D = new Proxy({} as CanvasRenderingContext2D, {
  get: (_t, prop) => (prop === "measureText" ? () => ({ width: 0 }) : () => {}),
  set: () => true
});
HTMLCanvasElement.prototype.getContext = (() => noop2d) as unknown as HTMLCanvasElement["getContext"];
window.matchMedia ??= (query: string) =>
  ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener() {},
    removeEventListener() {},
    addListener() {},
    removeListener() {},
    dispatchEvent: () => false
  }) as MediaQueryList;
Element.prototype.scrollIntoView ??= function () {};
Element.prototype.hasPointerCapture ??= () => false;
Element.prototype.releasePointerCapture ??= function () {};
Element.prototype.setPointerCapture ??= function () {};

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});
