-- Execute as the migration owner after cmd/migrate, never from API startup.
-- This file contains no password. A newly created role cannot log in until
-- its password is set privately with psql \password nsm_api and LOGIN enabled.
BEGIN;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'nsm_api') THEN
        CREATE ROLE nsm_api NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE
            NOREPLICATION NOBYPASSRLS;
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_roles
        WHERE rolname = 'nsm_api'
          AND (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls)
    ) OR EXISTS (
        SELECT 1 FROM pg_auth_members
        WHERE member = (SELECT oid FROM pg_roles WHERE rolname = 'nsm_api')
    ) OR EXISTS (
        SELECT 1 FROM pg_namespace
        WHERE nspname = 'app'
          AND nspowner = (SELECT oid FROM pg_roles WHERE rolname = 'nsm_api')
    ) OR EXISTS (
        SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'app'
          AND c.relowner = (SELECT oid FROM pg_roles WHERE rolname = 'nsm_api')
    ) THEN
        RAISE EXCEPTION 'nsm_api must be unprivileged, without memberships or ownership of app objects';
    END IF;
    EXECUTE format('GRANT CONNECT ON DATABASE %I TO nsm_api', current_database());
END $$;

REVOKE ALL ON SCHEMA app FROM nsm_api;
REVOKE ALL ON ALL TABLES IN SCHEMA app FROM nsm_api;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA app FROM nsm_api;
GRANT USAGE ON SCHEMA app TO nsm_api;

GRANT SELECT, INSERT, UPDATE ON app.users, app.recovery_codes,
    app.verification_challenges, app.social_profiles TO nsm_api;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.sessions, app.rate_limit_buckets TO nsm_api;
GRANT SELECT, INSERT ON app.instagram_message_receipts TO nsm_api;
GRANT INSERT ON app.audit_events TO nsm_api;
GRANT USAGE ON SEQUENCE app.recovery_codes_id_seq, app.audit_events_id_seq TO nsm_api;

COMMIT;
