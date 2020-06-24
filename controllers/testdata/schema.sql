-- Create tables
create table test(id serial, name text);
create table "Reply"(id serial, name text);
create table test2(name text, number integer);
create table test3(id serial, name text UNIQUE);
create table test4(id serial primary key, name text UNIQUE);
create table test5(id serial, name text, celphone text);
create table test6(nuveo serial primary key, name text, celphone text);
create table test7(id serial, name text, surname text);
create table test8(nameforjoin text);
create table test_readonly_access(id serial, name text);
create table test_write_and_delete_access(id serial, name text);
create table test_list_only_id(id serial, name text);
create table test_deleteonly_access(id serial, name text);
create table test_jsonb_bug(id serial NOT NULL, name text, data jsonb);
create table testjson(name text, data jsonb);
create table testarray(id serial, data character varying(250)[]);
create table test_empty_table(id serial, data character varying(250)[]);
create table test_group_by_table(id serial, name text, age integer, salary int);

-- Inserts
insert into test (name) values ('prest tester');
insert into "Reply" (name) values ('prest tester');
insert into test (name) values ('tester02');
insert into test2 (name, number) values ('tester02', 2);
insert into test3 (name) values ('prest');
insert into test3 (name) values ('prest tester');
insert into test5 (name, celphone) values ('prest tester', '444444');
insert into test7 (name, surname) values ('gopher', 'da silva');
insert into test_readonly_access (name) values ('test01');
insert into test_write_and_delete_access (name) values ('test01');
insert into test_list_only_id (name) values ('test01');
insert into test_deleteonly_access (name) values ('test01');
insert into test_jsonb_bug (name, data) values('goku', '{"techniques": ["kamehameha", "kaioken", "genki-dama"]}');
insert into test_jsonb_bug (name, data) values('goku', '{"description": "testing"}');
insert into testarray (id, data) values(100, '{"Gohan","Goten"}');

insert into test_group_by_table(name, age, salary) values ('joao', 20, 1250);
insert into test_group_by_table(name, age, salary) values ('maria', 19, 3999);
insert into test_group_by_table(name, age, salary) values ('gopher', 20, 100);
insert into test_group_by_table(name, age, salary) values ('guitarra humana', 19, 3998);

-- Views
create table table_to_view(id serial, name text, celphone text);
insert into table_to_view (name, celphone) values ('gopher', '8888888');
create view view_test as select name as player from table_to_view;