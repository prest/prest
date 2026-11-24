-- Same DDL as app/schema.go EnsureAuthTable, so concurrent auth instances never race on creation.
CREATE TABLE IF NOT EXISTS public.prest_users (id serial PRIMARY KEY, name text, username text UNIQUE, password text, metadata jsonb);
