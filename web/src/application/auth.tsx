import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  PlatformError,
  type PlatformGateway,
  type SessionView,
} from "../domain/platform";
import { carryGuestDraft, clearPrivateStorage } from "./private-storage";

interface AuthContextValue {
  gateway: PlatformGateway;
  session: SessionView | null;
  loading: boolean;
  connectionError: boolean;
  refresh(): Promise<void>;
  login(input: Parameters<PlatformGateway["login"]>[0]): Promise<void>;
  logout(): Promise<void>;
}
const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({
  gateway,
  children,
}: {
  gateway: PlatformGateway;
  children: ReactNode;
}) {
  const [session, setSession] = useState<SessionView | null>(null);
  const [loading, setLoading] = useState(true);
  const [connectionError, setConnectionError] = useState(false);
  const revision = useRef(0);
  const refresh = useCallback(
    async (signal?: AbortSignal, carryGuest = false) => {
      const requestRevision = ++revision.current;
      try {
        const value = await gateway.current(signal);
        if (signal?.aborted || requestRevision !== revision.current) return;
        if (carryGuest) carryGuestDraft(value.user.id);
        setSession(value);
        setConnectionError(false);
      } catch (error) {
        if (signal?.aborted || requestRevision !== revision.current) return;
        if (error instanceof PlatformError && error.status === 401) {
          clearPrivateStorage();
          setSession(null);
          setConnectionError(false);
        } else {
          setConnectionError(true);
          throw error;
        }
      } finally {
        if (!signal?.aborted && requestRevision === revision.current)
          setLoading(false);
      }
    },
    [gateway],
  );
  useEffect(() => {
    const controller = new AbortController();
    void refresh(controller.signal).catch(() => {});
    return () => controller.abort();
  }, [refresh]);
  const login = useCallback(
    async (input: Parameters<PlatformGateway["login"]>[0]) => {
      ++revision.current;
      try {
        await gateway.login(input);
        await refresh(undefined, true);
      } finally {
        setLoading(false);
      }
    },
    [gateway, refresh],
  );
  const logout = useCallback(async () => {
    ++revision.current;
    await gateway.logout();
    // Invalidate refreshes started while the logout request was in flight.
    ++revision.current;
    clearPrivateStorage(true);
    setSession(null);
    setConnectionError(false);
  }, [gateway]);
  const value = useMemo(
    () => ({
      gateway,
      session,
      loading,
      connectionError,
      refresh,
      login,
      logout,
    }),
    [gateway, session, loading, connectionError, refresh, login, logout],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("AuthProvider is required");
  return value;
}
