import { vi } from "vitest";

// jsdom has no WebAuthn. Options pass through unchanged and the "credential"
// serializes to a fixed JSON body, so tests can assert what reaches the API.
export function stubWebAuthn(credential: object | null = { id: "cred-1" }) {
  vi.stubGlobal("PublicKeyCredential", {
    parseCreationOptionsFromJSON: (o: unknown) => o,
    parseRequestOptionsFromJSON: (o: unknown) => o
  });
  const result = credential && { toJSON: () => credential };
  const credentials = { create: vi.fn(async () => result), get: vi.fn(async () => result) };
  Object.defineProperty(navigator, "credentials", { value: credentials, configurable: true });
  return credentials;
}
