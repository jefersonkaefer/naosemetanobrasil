import { useState } from "react";
import { messageFor, PlatformError } from "../domain/platform";
import { useAuth } from "./auth";

export function useAction() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const auth = useAuth();
  async function run(task: () => Promise<void>) {
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      await task();
    } catch (cause) {
      setError(messageFor(cause));
      if (cause instanceof PlatformError && cause.status === 401)
        void auth.refresh().catch(() => {});
    } finally {
      setBusy(false);
    }
  }
  return { busy, error, run, setError };
}
