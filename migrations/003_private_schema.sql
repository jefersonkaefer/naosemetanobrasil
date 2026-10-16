-- Do not rely on provider defaults when the database is hosted on Supabase.
-- The Go runtime receives explicit grants through its dedicated database role.
REVOKE ALL ON SCHEMA app FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA app FROM PUBLIC;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA app FROM PUBLIC;
DO $$ DECLARE role_name text; BEGIN
    FOREACH role_name IN ARRAY ARRAY['anon','authenticated'] LOOP
        IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname=role_name) AND role_name<>current_user THEN
            EXECUTE format('REVOKE ALL ON SCHEMA app FROM %I',role_name);
            EXECUTE format('REVOKE ALL ON ALL TABLES IN SCHEMA app FROM %I',role_name);
            EXECUTE format('REVOKE ALL ON ALL SEQUENCES IN SCHEMA app FROM %I',role_name);
        END IF;
    END LOOP;
END $$;
