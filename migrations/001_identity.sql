CREATE SCHEMA IF NOT EXISTS app;

CREATE TABLE app.users (
    id uuid PRIMARY KEY,
    username text NOT NULL UNIQUE CHECK (username ~ '^[a-z0-9_]{3,32}$'),
    display_name text NOT NULL DEFAULT '' CHECK (length(display_name) <= 80),
    password_hash text NOT NULL,
    role text NOT NULL CHECK (role IN ('member', 'admin')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    totp_encrypted bytea,
    totp_last_step bigint NOT NULL DEFAULT -1,
    created_at timestamptz NOT NULL,
    CHECK (role <> 'admin' OR totp_encrypted IS NOT NULL)
);
CREATE TABLE app.sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES app.users(id),
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz
);
CREATE INDEX sessions_user ON app.sessions(user_id);
CREATE TABLE app.recovery_codes (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES app.users(id),
    code_hash text NOT NULL UNIQUE,
    used_at timestamptz
);
CREATE INDEX recovery_user ON app.recovery_codes(user_id);
CREATE TABLE app.verification_challenges (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES app.users(id),
    expected_handle text NOT NULL CHECK (length(expected_handle) BETWEEN 1 AND 30),
    code_hash text NOT NULL UNIQUE,
    status text NOT NULL CHECK (status IN ('pending','awaiting_confirmation','verified','expired','rejected','cancelled')),
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL CHECK (expires_at > created_at),
    sent_at timestamptz,
    observed_handle text NOT NULL DEFAULT '',
    reviewed_by uuid REFERENCES app.users(id),
    review_reference text NOT NULL DEFAULT '',
    consumed_at timestamptz,
    CHECK (status NOT IN ('awaiting_confirmation','verified') OR (observed_handle = expected_handle AND reviewed_by IS NOT NULL)),
    CHECK ((status = 'verified') = (consumed_at IS NOT NULL))
);
CREATE UNIQUE INDEX challenge_one_pending ON app.verification_challenges(user_id)
    WHERE status IN ('pending','awaiting_confirmation');
CREATE INDEX challenge_queue ON app.verification_challenges(created_at, id)
    WHERE status IN ('pending','awaiting_confirmation');
CREATE TABLE app.social_profiles (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES app.users(id),
    platform text NOT NULL CHECK (platform = 'instagram'),
    handle text NOT NULL CHECK (length(handle) BETWEEN 1 AND 30),
    method text NOT NULL CHECK (method = 'manual_dm'),
    verified_at timestamptz NOT NULL,
    revoked_at timestamptz,
    public_visibility boolean NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX profile_unique_handle ON app.social_profiles(platform, handle) WHERE revoked_at IS NULL;
CREATE UNIQUE INDEX profile_unique_owner ON app.social_profiles(user_id, platform) WHERE revoked_at IS NULL;
CREATE TABLE app.audit_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_id uuid NOT NULL REFERENCES app.users(id),
    action text NOT NULL,
    entity_id uuid NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL
);
CREATE TABLE app.rate_limit_buckets (
    key_hash text NOT NULL,
    window_start timestamptz NOT NULL,
    count integer NOT NULL CHECK (count > 0),
    expires_at timestamptz NOT NULL,
    PRIMARY KEY(key_hash, window_start)
);
CREATE INDEX rate_limit_expiration ON app.rate_limit_buckets(expires_at);

-- The schema is intentionally not exposed through the Supabase Data API.
REVOKE ALL ON SCHEMA app FROM PUBLIC;
