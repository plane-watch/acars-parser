-- Creates or updates the read-only PostgreSQL role that the dbviewer service
-- (pgweb) connects as. See docs/deployment.md.
--
-- The script is idempotent: running it again resets the password and
-- re-applies the grants, so it is also the way to change the password and to
-- cover tables created since the last run.
--
-- It must run as the owner of the tables (the acars role created by the
-- postgres image), because the default privileges below apply to the tables
-- that role creates later. psql supplies two variables:
--   viewer_role      the name of the role, e.g. acars_viewer
--   viewer_password  the password of the role
--
-- Run it with the one-off dbviewer-role service:
--   docker compose -f deployments/docker-compose.yml run --rm dbviewer-role

\set ON_ERROR_STOP on

-- Create the role only if it does not exist. CREATE ROLE has no IF NOT EXISTS,
-- so the statement is built here and run by \gexec only when no row matches.
SELECT format('CREATE ROLE %I LOGIN', :'viewer_role')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'viewer_role')
\gexec

-- The attributes are set explicitly so that a pre-existing role of the same
-- name loses any extra powers. NOINHERIT stops the role from using the
-- privileges of any role it might later be made a member of.
ALTER ROLE :"viewer_role" WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT
    PASSWORD :'viewer_password';

-- Every transaction of the role starts read-only. The role could switch this
-- off for its own session, so it is a second safeguard, not the main one; the
-- main one is that the role holds no privilege other than SELECT.
ALTER ROLE :"viewer_role" SET default_transaction_read_only = on;

GRANT CONNECT ON DATABASE acars_state TO :"viewer_role";
GRANT USAGE ON SCHEMA public TO :"viewer_role";
GRANT SELECT ON ALL TABLES IN SCHEMA public TO :"viewer_role";

-- Tables that the acars role creates later are readable too.
ALTER DEFAULT PRIVILEGES FOR ROLE acars IN SCHEMA public
    GRANT SELECT ON TABLES TO :"viewer_role";
