import { PostRequest } from "@/util";
import { toast } from "sonner";

// The backend (go-webauthn) speaks the WebAuthn JSON format; browsers convert it
// natively (Chrome 129+, Firefox 119+, Safari 18.4+).
export function passkeysSupported() {
  return (
    typeof PublicKeyCredential !== "undefined" &&
    typeof PublicKeyCredential.parseCreationOptionsFromJSON === "function"
  );
}

// The user dismissing the browser prompt is not an error worth a toast.
function reportCeremonyError(err: unknown) {
  if (err instanceof DOMException && err.name === "NotAllowedError") return;
  toast.error("Passkey operation failed", {
    description: err instanceof Error ? err.message : String(err)
  });
}

export async function createPasskey(name: string) {
  const [status, begin] = await PostRequest("auth/webauthn/register/begin", { name });
  if (status !== 200) return null;
  try {
    const publicKey = PublicKeyCredential.parseCreationOptionsFromJSON(begin.options.publicKey);
    const credential = (await navigator.credentials.create({ publicKey })) as PublicKeyCredential | null;
    if (!credential) return null;
    const [finishStatus, finish] = await PostRequest(
      `auth/webauthn/register/finish?sessionId=${encodeURIComponent(begin.sessionId)}`,
      credential.toJSON()
    );
    return finishStatus === 200 ? (finish as { id: number; name: string }) : null;
  } catch (err) {
    reportCeremonyError(err);
    return null;
  }
}

export async function loginWithPasskey(username: string) {
  const [status, begin] = await PostRequest("auth/webauthn/login/begin", { username }, true);
  if (status !== 200) return null;
  try {
    const publicKey = PublicKeyCredential.parseRequestOptionsFromJSON(begin.options.publicKey);
    const credential = (await navigator.credentials.get({ publicKey })) as PublicKeyCredential | null;
    if (!credential) return null;
    const [finishStatus, finish] = await PostRequest(
      `auth/webauthn/login/finish?sessionId=${encodeURIComponent(begin.sessionId)}`,
      credential.toJSON(),
      true
    );
    return finishStatus === 200 ? (finish as { username: string; role: string }) : null;
  } catch (err) {
    reportCeremonyError(err);
    return null;
  }
}
