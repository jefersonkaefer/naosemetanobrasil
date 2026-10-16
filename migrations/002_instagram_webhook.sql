ALTER TABLE app.verification_challenges
    ADD COLUMN method text NOT NULL DEFAULT 'manual_dm' CHECK (method IN ('manual_dm','instagram_webhook')),
    ADD COLUMN observed_account_id text NOT NULL DEFAULT '',
    ADD COLUMN observed_sender_id text NOT NULL DEFAULT '';

-- Replace the original proof constraint by its definition, without depending
-- on the generated name PostgreSQL chose for the unnamed check.
DO $$
DECLARE constraint_name text;
BEGIN
    FOR constraint_name IN
        SELECT conname FROM pg_constraint
        WHERE conrelid='app.verification_challenges'::regclass AND contype='c'
        AND pg_get_constraintdef(oid) LIKE '%reviewed_by IS NOT NULL%'
    LOOP
        EXECUTE format('ALTER TABLE app.verification_challenges DROP CONSTRAINT %I', constraint_name);
    END LOOP;
END $$;

ALTER TABLE app.verification_challenges ADD CONSTRAINT challenge_identity_proof CHECK (
    status NOT IN ('awaiting_confirmation','verified') OR (
        observed_handle=expected_handle AND (
            (method='manual_dm' AND reviewed_by IS NOT NULL) OR
            (method='instagram_webhook' AND reviewed_by IS NULL AND observed_account_id ~ '^[0-9]{1,32}$' AND observed_sender_id ~ '^[0-9]{1,32}$')
        )
    )
);

ALTER TABLE app.social_profiles DROP CONSTRAINT social_profiles_method_check;
ALTER TABLE app.social_profiles
    ADD CONSTRAINT social_profiles_method_check CHECK (method IN ('manual_dm','instagram_webhook')),
    ADD COLUMN remote_account_id text NOT NULL DEFAULT '',
    ADD COLUMN remote_user_id text NOT NULL DEFAULT '',
    ADD CONSTRAINT profile_remote_identity CHECK (method <> 'instagram_webhook' OR (remote_account_id ~ '^[0-9]{1,32}$' AND remote_user_id ~ '^[0-9]{1,32}$'));
CREATE UNIQUE INDEX profile_unique_remote_identity ON app.social_profiles(platform,remote_account_id,remote_user_id)
    WHERE revoked_at IS NULL AND remote_user_id <> '';

CREATE TABLE app.instagram_message_receipts (
    account_id text NOT NULL,
    message_id text NOT NULL CHECK (length(message_id) BETWEEN 1 AND 512),
    challenge_id uuid NOT NULL REFERENCES app.verification_challenges(id),
    received_at timestamptz NOT NULL,
    PRIMARY KEY(account_id,message_id)
);
-- No raw message text, verification codes or API tokens are stored here.
