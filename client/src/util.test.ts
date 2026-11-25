import { mockApi } from "@/test/api";
import { describe, expect, it, vi } from "vitest";
import { DeleteRequest, GetRequest, PatchRequest, PostRequest, PutRequest, getApiBaseUrl } from "./util";

describe("request helpers", () => {
  it("default to localhost:8080 and honour SERVER_CONFIG.port", () => {
    expect(getApiBaseUrl()).toBe("http://localhost:8080");
    window.SERVER_CONFIG = { port: 9999 };
    expect(getApiBaseUrl()).toMatch(/:9999$/);
    delete window.SERVER_CONFIG;
  });

  it.each([
    ["GET", (u: string) => GetRequest(u)],
    ["POST", (u: string) => PostRequest(u, { a: 1 })],
    ["PUT", (u: string) => PutRequest(u, { a: 1 })],
    ["PATCH", (u: string) => PatchRequest(u)],
    ["DELETE", (u: string) => DeleteRequest(u, { a: 1 })]
  ])("%s returns [status, json] on success", async (method, call) => {
    const { calls } = mockApi({ [`${method} thing`]: { ok: true } });
    expect(await call("thing")).toEqual([200, { ok: true }]);
    expect(calls[0]).toMatchObject({ method, path: "thing" });
  });

  it.each([
    ["GET", (u: string) => GetRequest(u, true)],
    ["POST", (u: string) => PostRequest(u, {}, false, true)],
    ["PUT", (u: string) => PutRequest(u, {}, true)],
    ["PATCH", (u: string) => PatchRequest(u, true)],
    ["DELETE", (u: string) => DeleteRequest(u, {}, true)]
  ])("%s returns the error status", async (method, call) => {
    mockApi({ [`${method} thing`]: () => [400, { error: "nope" }] });
    const [status] = await call("thing");
    expect(status).toBe(400);
  });

  it("returns the error message on 401 (jsdom cannot follow the /login redirect)", async () => {
    mockApi({ secret: () => [401, { error: "unauthorized" }] });
    expect(await GetRequest("secret", true)).toEqual([401, "unauthorized"]);
  });

  it("returns 500 when the server is unreachable", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("offline")));
    expect(await GetRequest("x")).toEqual([500, null]);
    expect(await PostRequest("x", {})).toEqual([500, null]);
  });
});
