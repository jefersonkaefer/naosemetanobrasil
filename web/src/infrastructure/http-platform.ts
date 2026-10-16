import {
  PlatformError,
  type Challenge,
  type IssuedChallenge,
  type PlatformGateway,
  type QueueItem,
  type Registration,
  type SessionView,
  type SocialProfile,
} from "../domain/platform";

type Transport = typeof fetch;
type RequestOptions = {
  method?: string;
  body?: unknown;
  protected?: boolean;
  signal?: AbortSignal;
};

export class HttpPlatform implements PlatformGateway {
  private csrf: string | null = null;
  private epoch = 0;
  constructor(
    private readonly transport: Transport = (...args) => fetch(...args),
  ) {}

  private async request<T>(
    path: string,
    options: RequestOptions = {},
  ): Promise<T> {
    const epoch = this.epoch;
    const headers: Record<string, string> = { Accept: "application/json" };
    if (options.body !== undefined)
      headers["Content-Type"] = "application/json";
    if (options.protected) {
      if (!this.csrf)
        throw new PlatformError(
          "unauthenticated",
          "Entre novamente para continuar.",
          401,
        );
      headers["X-CSRF-Token"] = this.csrf;
    }
    const controller = new AbortController();
    const onAbort = () => controller.abort();
    if (options.signal?.aborted) controller.abort();
    else options.signal?.addEventListener("abort", onAbort, { once: true });
    const timeout = setTimeout(() => controller.abort(), 75_000);
    try {
      let response: Response;
      try {
        response = await this.transport(path, {
          method: options.method || "GET",
          headers,
          credentials: "same-origin",
          cache: "no-store",
          body:
            options.body === undefined
              ? undefined
              : JSON.stringify(options.body),
          signal: controller.signal,
        });
      } catch (error) {
        if (options.signal?.aborted) throw error;
        throw new PlatformError(
          "unavailable",
          "A conexão demorou ou foi interrompida. Tente novamente.",
        );
      }
      if (response.status === 204) return undefined as T;
      let data: unknown;
      try {
        data = await response.json();
      } catch {
        throw new PlatformError(
          "unavailable",
          "A plataforma está iniciando ou temporariamente indisponível. Aguarde e tente novamente.",
          response.status,
        );
      }
      if (!response.ok) {
        const detail = (data as { error?: { code?: string; message?: string } })
          ?.error;
        if (response.status === 401 && epoch === this.epoch) this.csrf = null;
        throw new PlatformError(
          detail?.code || "unavailable",
          detail?.message || "Não foi possível concluir. Tente novamente.",
          response.status,
        );
      }
      return data as T;
    } finally {
      clearTimeout(timeout);
      options.signal?.removeEventListener("abort", onAbort);
    }
  }

  async current(signal?: AbortSignal): Promise<SessionView> {
    const epoch = this.epoch;
    const result = await this.request<SessionView & { csrf_token: string }>(
      "/v1/me",
      { signal },
    );
    if (epoch === this.epoch) this.rememberCSRF(result.csrf_token);
    return { user: result.user, instagram_profile: result.instagram_profile };
  }
  register(input: Parameters<PlatformGateway["register"]>[0]) {
    return this.request<Registration>("/v1/auth/register", {
      method: "POST",
      body: input,
    });
  }
  async login(input: Parameters<PlatformGateway["login"]>[0]) {
    ++this.epoch;
    const result = await this.request<{ csrf_token: string }>(
      "/v1/auth/login",
      { method: "POST", body: input },
    );
    this.rememberCSRF(result.csrf_token);
  }
  async logout() {
    await this.request("/v1/auth/logout", { method: "POST", protected: true });
    ++this.epoch;
    this.csrf = null;
  }
  async recover(input: Parameters<PlatformGateway["recover"]>[0]) {
    await this.request("/v1/auth/recover", { method: "POST", body: input });
    ++this.epoch;
    this.csrf = null;
  }
  issueChallenge(handle: string) {
    return this.request<IssuedChallenge>("/v1/me/instagram/challenges", {
      method: "POST",
      body: { handle },
      protected: true,
    });
  }
  challenge(id: string, signal?: AbortSignal) {
    return this.request<Challenge>(
      `/v1/me/instagram/challenges/${encodeURIComponent(id)}`,
      { signal },
    );
  }
  async markSent(id: string) {
    await this.request(
      `/v1/me/instagram/challenges/${encodeURIComponent(id)}/sent`,
      { method: "POST", protected: true },
    );
  }
  confirm(id: string) {
    return this.request<SocialProfile>(
      `/v1/me/instagram/challenges/${encodeURIComponent(id)}/confirm`,
      { method: "POST", protected: true },
    );
  }
  async revokeProfile(id: string) {
    await this.request(`/v1/me/social-profiles/${encodeURIComponent(id)}`, {
      method: "DELETE",
      protected: true,
    });
  }
  queue() {
    return this.request<{ items: QueueItem[]; limit: number }>(
      "/v1/admin/verifications",
    );
  }
  observe(input: Parameters<PlatformGateway["observe"]>[0]) {
    return this.request<Challenge>("/v1/admin/verifications/observe", {
      method: "POST",
      body: input,
      protected: true,
    });
  }
  async reject(id: string, reason: string) {
    await this.request(
      `/v1/admin/verifications/${encodeURIComponent(id)}/reject`,
      { method: "POST", body: { reason }, protected: true },
    );
  }

  private rememberCSRF(value: string) {
    if (typeof value !== "string" || !/^[0-9a-f]{64}$/.test(value))
      throw new PlatformError(
        "unavailable",
        "Não foi possível iniciar uma sessão segura. Tente novamente.",
      );
    this.csrf = value;
  }
}
