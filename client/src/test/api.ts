import { vi } from "vitest";
import { apiDefaults } from "./fixtures";

type Reply = unknown | ((body: unknown) => [number, unknown]);

// Stubs fetch for /api/<path>. A route value is either the JSON body (200) or a
// function returning [status, body]. Unknown routes answer 200 with no body;
// common endpoints default to the fixtures in ./fixtures.
export function mockApi(overrides: Record<string, Reply> = {}) {
  const routes = { ...apiDefaults, ...overrides };
  const calls: { method: string; path: string; body: unknown }[] = [];
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input).replace(/^.*\/api\//, "");
    const method = init?.method ?? "GET";
    const body = typeof init?.body === "string" ? JSON.parse(init.body) : init?.body;
    calls.push({ method, path, body });

    const bare = path.split("?")[0];
    const key = [`${method} ${path}`, `${method} ${bare}`, path, bare].find((k) => k in routes);
    const route = key === undefined ? undefined : routes[key];
    const [status, data] =
      typeof route === "function" ? route(body) : [200, route ?? null];
    const text = data === null || data === undefined ? "" : JSON.stringify(data);
    return new Response(text, { status, headers: { "Content-Type": "application/json" } });
  });
  vi.stubGlobal("fetch", fetchMock);
  return { calls, fetchMock };
}
