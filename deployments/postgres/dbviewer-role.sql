-- Creates or updates the read-only PostgreSQL role that the dbviewer service
-- (pgweb) connects as. See docs/deployment.md.
--
-- The script is idempotent: running it again resets the password and
-- re-applies the grants, so it is also the way to change the password and to
-- cover tables created since the last run.
--
-- It must run as the acars role (a superuser in the postgres image): the
-- default privileges below apply to the tables that role creates later, and
-- revoking from PUBLIC on pg_catalog functions needs a superuser.
--
-- The role name comes from the psql variable viewer_role (e.g.
-- acars_viewer) and the password from the environment variable
-- VIEWER_PASSWORD, read with \getenv so that it never appears in a command
-- line. Run it with the one-off dbviewer-role service:
--   docker compose -f deployments/docker-compose.yml run --rm dbviewer-role

\set ON_ERROR_STOP on
\getenv viewer_password VIEWER_PASSWORD

-- Create the role only if it does not exist. CREATE ROLE has no IF NOT EXISTS,
-- so the statement is built here and run by \gexec only when no row matches.
SELECT format('CREATE ROLE %I LOGIN', :'viewer_role')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'viewer_role')
\gexec

-- A role that owns objects can change them whatever its grants, so such a
-- role is refused rather than reconciled. The query always returns one row;
-- a NULL reason leaves the psql variable unset.
SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_class WHERE relowner = r.oid)
              OR EXISTS (SELECT 1 FROM pg_namespace WHERE nspowner = r.oid)
              OR EXISTS (SELECT 1 FROM pg_proc WHERE proowner = r.oid)
              OR EXISTS (SELECT 1 FROM pg_largeobject_metadata WHERE lomowner = r.oid)
       THEN format('the role %I owns database objects; refusing to make it the read-only viewer', r.rolname)
       END AS reason
FROM pg_roles r WHERE r.rolname = :'viewer_role'
\gset refused_
\if :{?refused_reason}
\echo :refused_reason
\quit 3
\endif

-- The attributes are set explicitly, so that a pre-existing role of the same
-- name loses any extra powers.
ALTER ROLE :"viewer_role" WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT
    PASSWORD :'viewer_password';

-- Remove any membership in other roles (a member can SET ROLE to them,
-- whatever NOINHERIT says) and any privilege granted earlier, then grant
-- only what reading needs.
SELECT format('REVOKE %I FROM %I', g.rolname, :'viewer_role')
FROM pg_auth_members m JOIN pg_roles g ON g.oid = m.roleid
WHERE m.member = (SELECT oid FROM pg_roles WHERE rolname = :'viewer_role')
\gexec
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM :"viewer_role";
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM :"viewer_role";
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA public FROM :"viewer_role";
REVOKE ALL ON SCHEMA public FROM :"viewer_role";
REVOKE ALL ON DATABASE acars_state FROM :"viewer_role";

GRANT CONNECT ON DATABASE acars_state TO :"viewer_role";
GRANT USAGE ON SCHEMA public TO :"viewer_role";
GRANT SELECT ON ALL TABLES IN SCHEMA public TO :"viewer_role";

-- Tables that the acars role creates later are readable too.
ALTER DEFAULT PRIVILEGES FOR ROLE acars IN SCHEMA public
    GRANT SELECT ON TABLES TO :"viewer_role";

-- PUBLIC, which every role belongs to, may by default create temporary
-- tables and large objects, neither of which needs a table privilege; a
-- large object persists. Neither is used by acars_parser, whose acars role
-- is a superuser and is unaffected.
REVOKE TEMPORARY ON DATABASE acars_state FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION pg_catalog.lo_create(oid) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION pg_catalog.lo_creat(integer) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION pg_catalog.lo_from_bytea(oid, bytea) FROM PUBLIC;

-- Every transaction of the role starts read-only. The role can switch this
-- off (SET default_transaction_read_only, or BEGIN READ WRITE), so it is
-- not a safeguard on its own: the safeguard is that the role holds no
-- privilege to change anything.
ALTER ROLE :"viewer_role" SET default_transaction_read_only = on;
