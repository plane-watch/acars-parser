-- Creates the read-only PostgreSQL role that the dbviewer service (pgweb)
-- connects as. See docs/deployment.md.
--
-- The role is dropped and created afresh on every run, so whatever an
-- earlier role of the same name could do (grants on tables, functions,
-- large objects or other schemas, default privileges, memberships) is
-- removed, and the new role holds only the grants below. Running it again
-- also changes the password and covers tables created since the last run.
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

-- An existing role that owns anything is refused, not dropped: DROP OWNED
-- would delete what it owns. The query returns one row if the role exists;
-- a NULL reason leaves the psql variable unset.
SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_database WHERE datdba = r.oid)
              OR EXISTS (SELECT 1 FROM pg_class WHERE relowner = r.oid)
              OR EXISTS (SELECT 1 FROM pg_namespace WHERE nspowner = r.oid)
              OR EXISTS (SELECT 1 FROM pg_proc WHERE proowner = r.oid)
              OR EXISTS (SELECT 1 FROM pg_type WHERE typowner = r.oid)
              OR EXISTS (SELECT 1 FROM pg_largeobject_metadata WHERE lomowner = r.oid)
       THEN format('the role %I owns database objects; refusing to replace it', r.rolname)
       END AS reason
FROM pg_roles r WHERE r.rolname = :'viewer_role'
\gset refused_
\if :{?refused_reason}
-- An SQL error, so that psql stops with a failure status.
SELECT format('DO $$BEGIN RAISE EXCEPTION %L; END$$', :'refused_reason')
\gexec
\endif

-- Remove an existing role and everything granted to it, then create it
-- afresh. DROP OWNED revokes its privileges (it owns nothing, as checked
-- above); DROP ROLE fails if anything still depends on it.
SELECT format('DROP OWNED BY %I', :'viewer_role'), format('DROP ROLE %I', :'viewer_role')
WHERE EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'viewer_role')
\gexec

CREATE ROLE :"viewer_role" WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT
    PASSWORD :'viewer_password';

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
-- off (BEGIN READ WRITE), so it is not a safeguard on its own: the
-- safeguard is that the role holds no privilege to change stored data.
ALTER ROLE :"viewer_role" SET default_transaction_read_only = on;
