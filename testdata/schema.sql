-- CREATE TABLES
CREATE TABLE test(id serial, name text);
CREATE TABLE "Reply"(id serial, name text);
CREATE TABLE "TestCase"(id serial, name text);
CREATE TABLE test2(name text, number integer);
CREATE TABLE test3(id serial, name text UNIQUE);
CREATE TABLE test4(id serial primary key, name text UNIQUE);
CREATE TABLE test5(id serial, name text, celphone text);
CREATE TABLE test6(id serial primary key, name text, celphone text);
CREATE TABLE test7(id serial, name text, surname text);
CREATE TABLE test8(nameforjoin text);
CREATE TABLE test_readonly_access(id serial, name text);
CREATE TABLE test_write_and_delete_access(id serial, name text);
CREATE TABLE test_list_only_id(id serial, name text);
CREATE TABLE test_deleteonly_access(id serial, name text);
CREATE TABLE test_jsonb_bug(id serial NOT NULL, name text, data jsonb);
CREATE TABLE testjson(name text, data jsonb);
CREATE TABLE testarray(id serial, data character varying(250)[]);
CREATE TABLE test_empty_table(id serial, data character varying(250)[]);
CREATE TABLE test_group_by_table(id serial, name text, age integer, salary int);
CREATE TABLE prest_users(id serial, username text, password text);

-- Inserts
INSERT INTO test (name) VALUES ('prest tester');
INSERT INTO "Reply" (name) VALUES ('prest tester');
INSERT INTO "TestCase" (name) VALUES ('prest tester');
INSERT INTO test (name) VALUES ('tester02');
INSERT INTO test2 (name, number) VALUES ('tester02', 2);
INSERT INTO test3 (name) VALUES ('prest');
INSERT INTO test3 (name) VALUES ('prest tester');
INSERT INTO test7 (name, surname) VALUES ('gopher', 'da silva');
INSERT INTO test_readonly_access (name) VALUES ('test01');
INSERT INTO test_write_and_delete_access (name) VALUES ('test01');
INSERT INTO test_list_only_id (name) VALUES ('test01');
INSERT INTO test_deleteonly_access (name) VALUES ('test01');
INSERT INTO test_jsonb_bug (name, data) VALUES('goku', '{"techniques": ["kamehameha", "kaioken", "genki-dama"]}');
INSERT INTO test_jsonb_bug (name, data) VALUES('goku', '{"description": "testing"}');
INSERT INTO testarray (id, data) VALUES(100, '{"Gohan","Goten"}');

INSERT INTO test_group_by_table(name, age, salary) VALUES('joao', 20, 1250);
INSERT INTO test_group_by_table(name, age, salary) VALUES('maria', 19, 3999);
INSERT INTO test_group_by_table(name, age, salary) VALUES('gopher', 20, 100);
INSERT INTO test_group_by_table(name, age, salary) VALUES('guitarra humana', 19, 3998);
INSERT INTO prest_users(username, password) VALUES('test@postgres.rest', 'e10adc3949ba59abbe56e057f20f883e');

-- Views
CREATE TABLE table_to_view(id serial, name text, celphone text);
INSERT INTO table_to_view (name, celphone) VALUES ('gopher', '8888888');
CREATE VIEW view_test AS SELECT name AS player from table_to_view;
