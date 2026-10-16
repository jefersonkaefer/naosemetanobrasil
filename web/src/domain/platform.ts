export interface User {
  id: string;
  username: string;
  display_name: string;
  role: "member" | "admin";
  created_at: string;
}

export interface SocialProfile {
  id: string;
  platform: "instagram";
  handle: string;
  verification_method: "manual_dm" | "instagram_webhook";
  verified_at: string;
  public_visibility: boolean;
}

export type ChallengeStatus =
  | "pending"
  | "awaiting_confirmation"
  | "verified"
  | "expired"
  | "rejected"
  | "cancelled";
export interface Challenge {
  id: string;
  expected_handle: string;
  observed_handle: string;
  status: ChallengeStatus;
  created_at: string;
  expires_at: string;
  sent_at: string | null;
  verification_method?: "manual_dm" | "instagram_webhook";
  verification_mode?: "manual" | "webhook";
}
export interface SessionView {
  user: User;
  instagram_profile: SocialProfile | null;
}
export interface Registration {
  user: User;
  recovery_codes: string[];
}
export interface IssuedChallenge {
  challenge: Challenge;
  code: string;
  official_handle: string;
  direct_url: string;
  verification_mode?: "manual" | "webhook";
}
export interface QueueItem extends Challenge {
  user_id: string;
}

// UI depends on this port. Transport, cookies and CSRF belong to the HTTP adapter.
export interface PlatformGateway {
  current(signal?: AbortSignal): Promise<SessionView>;
  register(input: {
    username: string;
    password: string;
    display_name: string;
  }): Promise<Registration>;
  login(input: {
    username: string;
    password: string;
    totp?: string;
  }): Promise<void>;
  logout(): Promise<void>;
  recover(input: {
    username: string;
    recovery_code: string;
    new_password: string;
  }): Promise<void>;
  issueChallenge(handle: string): Promise<IssuedChallenge>;
  challenge(id: string, signal?: AbortSignal): Promise<Challenge>;
  markSent(id: string): Promise<void>;
  confirm(id: string): Promise<SocialProfile>;
  revokeProfile(id: string): Promise<void>;
  queue(): Promise<{ items: QueueItem[]; limit: number }>;
  observe(input: {
    code: string;
    observed_handle: string;
    reference: string;
  }): Promise<Challenge>;
  reject(id: string, reason: string): Promise<void>;
}

export class PlatformError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly status = 0,
  ) {
    super(message);
    this.name = "PlatformError";
  }
}

export function messageFor(error: unknown): string {
  return error instanceof PlatformError
    ? error.message
    : "Não foi possível concluir. Tente novamente.";
}

export function activeStatus(
  challenge: Challenge,
  now = Date.now(),
): ChallengeStatus {
  if (
    ["pending", "awaiting_confirmation"].includes(challenge.status) &&
    Date.parse(challenge.expires_at) <= now
  )
    return "expired";
  return challenge.status;
}
