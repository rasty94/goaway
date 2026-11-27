import { mockApi } from "@/test/api";
import { stubWebAuthn } from "@/test/webauthn";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { PasskeysRow, TwoFactorRow } from "./AccountSecurity";

describe("TwoFactorRow", () => {
  it("enables 2FA: generates a key, shows the QR and verifies a code", async () => {
    const { calls } = mockApi({
      "auth/2fa/status": { enabled: false },
      "POST auth/2fa/setup": { secret: "JBSWY3DPEHPK3PXP", otpauthUrl: "otpauth://totp/GoAway:admin?secret=JBSWY3DPEHPK3PXP" },
      "POST auth/2fa/enable": { enabled: true }
    });
    const user = userEvent.setup();
    render(<TwoFactorRow />);

    await user.click(await screen.findByRole("button", { name: "Enable 2FA" }));
    await user.click(screen.getByRole("button", { name: "Generate setup key" }));

    expect(await screen.findByTestId("totp-secret")).toHaveTextContent("JBSWY3DPEHPK3PXP");
    expect(screen.getByTitle("2FA QR code")).toBeInTheDocument();

    const verify = screen.getByRole("button", { name: "Verify and enable" });
    expect(verify).toBeDisabled();
    await user.type(screen.getByLabelText("6-digit code"), "123456");
    await user.click(verify);

    expect(await screen.findByRole("button", { name: "Disable 2FA" })).toBeInTheDocument();
    expect(calls.find((c) => c.path === "auth/2fa/enable")?.body).toEqual({ code: "123456" });
  });

  it("keeps 2FA off when the code is wrong", async () => {
    mockApi({
      "auth/2fa/status": { enabled: false },
      "POST auth/2fa/setup": { secret: "S", otpauthUrl: "otpauth://totp/GoAway:admin?secret=S" },
      "POST auth/2fa/enable": () => [401, { error: "invalid 2FA code" }]
    });
    const user = userEvent.setup();
    render(<TwoFactorRow />);

    await user.click(await screen.findByRole("button", { name: "Enable 2FA" }));
    await user.click(screen.getByRole("button", { name: "Generate setup key" }));
    await user.type(await screen.findByLabelText("6-digit code"), "000000");
    await user.click(screen.getByRole("button", { name: "Verify and enable" }));

    await waitFor(() => expect(screen.getByLabelText("6-digit code")).toBeInTheDocument());
    expect(screen.getByText(/Require a code/)).toBeInTheDocument();
  });

  it("disables 2FA with a current code", async () => {
    const { calls } = mockApi({ "auth/2fa/status": { enabled: true }, "POST auth/2fa/disable": { enabled: false } });
    const user = userEvent.setup();
    render(<TwoFactorRow />);

    await user.click(await screen.findByRole("button", { name: "Disable 2FA" }));
    await user.type(screen.getByLabelText("6-digit code"), "654321");
    await user.click(screen.getAllByRole("button", { name: "Disable 2FA" }).at(-1)!);

    expect(await screen.findByRole("button", { name: "Enable 2FA" })).toBeInTheDocument();
    expect(calls.find((c) => c.path === "auth/2fa/disable")?.body).toEqual({ code: "654321" });
  });
});

describe("PasskeysRow", () => {
  it("is hidden when WebAuthn is disabled on the server", async () => {
    stubWebAuthn();
    const { calls } = mockApi({ "auth/webauthn/status": { enabled: false } });
    const { container } = render(<PasskeysRow />);
    await waitFor(() => expect(calls.some((c) => c.path === "auth/webauthn/status")).toBe(true));
    expect(container).toBeEmptyDOMElement();
  });

  it("lists, registers and removes passkeys", async () => {
    const credentials = stubWebAuthn({ id: "new-cred", type: "public-key" });
    let keys = [{ id: 1, name: "YubiKey", createdAt: "2026-11-20T10:00:00Z" }];
    const { calls } = mockApi({
      "auth/webauthn/status": { enabled: true },
      "auth/webauthn/credentials": () => [200, keys],
      "POST auth/webauthn/register/begin": { sessionId: "s1", options: { publicKey: { challenge: "abc" } } },
      "POST auth/webauthn/register/finish": () => {
        keys = [...keys, { id: 2, name: "MacBook", createdAt: "2026-11-27T10:00:00Z" }];
        return [200, { id: 2, name: "MacBook" }];
      },
      "DELETE auth/webauthn/credentials/1": { deleted: 1 }
    });
    const user = userEvent.setup();
    render(<PasskeysRow />);

    await user.click(await screen.findByRole("button", { name: "Manage Passkeys" }));
    expect(await screen.findByText("YubiKey")).toBeInTheDocument();

    await user.type(screen.getByLabelText("Passkey name"), "MacBook");
    await user.click(screen.getByRole("button", { name: "Add passkey" }));
    expect(await screen.findByText("MacBook")).toBeInTheDocument();
    expect(credentials.create).toHaveBeenCalledWith({ publicKey: { challenge: "abc" } });
    expect(calls.find((c) => c.path.startsWith("auth/webauthn/register/finish"))).toMatchObject({
      path: "auth/webauthn/register/finish?sessionId=s1",
      body: { id: "new-cred", type: "public-key" }
    });

    await user.click(screen.getByRole("button", { name: "Remove YubiKey" }));
    await waitFor(() => expect(screen.queryByText("YubiKey")).not.toBeInTheDocument());
  });

  it("does not call finish when the user cancels the browser prompt", async () => {
    const credentials = stubWebAuthn();
    credentials.create.mockRejectedValueOnce(new DOMException("cancelled", "NotAllowedError"));
    const { calls } = mockApi({
      "auth/webauthn/status": { enabled: true },
      "auth/webauthn/credentials": [],
      "POST auth/webauthn/register/begin": { sessionId: "s1", options: { publicKey: {} } }
    });
    const user = userEvent.setup();
    render(<PasskeysRow />);

    await user.click(await screen.findByRole("button", { name: "Manage Passkeys" }));
    await user.click(screen.getByRole("button", { name: "Add passkey" }));
    await waitFor(() => expect(credentials.create).toHaveBeenCalled());
    expect(calls.some((c) => c.path.startsWith("auth/webauthn/register/finish"))).toBe(false);
    expect(calls.find((c) => c.path === "auth/webauthn/register/begin")?.body).toEqual({ name: "Passkey" });
  });
});
