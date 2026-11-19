-- Deterministic Studio e2e fixture (no now()); applied to both `prest` and `prest_auth`.
CREATE SCHEMA IF NOT EXISTS shop;

CREATE TABLE public.items (
  id serial PRIMARY KEY,
  name text NOT NULL,
  qty integer,
  price numeric(10, 2),
  active boolean,
  created timestamptz,
  meta jsonb,
  tags text[],
  note text
);
INSERT INTO public.items (name, qty, price, active, created, meta, tags, note)
SELECT 'item-' || g, g, g * 1.5, g % 2 = 0,
       timestamptz '2026-01-01T00:00:00Z' + g * interval '1 minute',
       jsonb_build_object('n', g), ARRAY['a', 'b'],
       CASE WHEN g % 5 = 0 THEN NULL ELSE 'note ' || g END
FROM generate_series(1, 73) AS g;

CREATE TABLE shop.orders (id serial PRIMARY KEY, customer text, total numeric);
INSERT INTO shop.orders (customer, total) VALUES ('alice', 10), ('bob', 20.5), (NULL, NULL);

CREATE VIEW public.active_items AS SELECT id, name FROM public.items WHERE active;

CREATE TABLE public."Mixed Case" (id serial, "Weird Col" text);
INSERT INTO public."Mixed Case" ("Weird Col") VALUES ('x');

CREATE TABLE public."TestCase" (id integer, label text);
INSERT INTO public."TestCase" VALUES (1, 'one');

CREATE TABLE public.big (id bigint, n numeric(30, 10));
INSERT INTO public.big VALUES (9007199254740993, 12345678901234567890.1234567891);
