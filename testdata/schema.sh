psql -c "DROP DATABASE IF EXISTS prest" -U postgres
psql -c "create database prest;" -U postgres
psql prest -c "create table test(id serial, name text);" -U postgres
psql prest -c "create table test2(name text, number integer);" -U postgres
psql prest -c "create table test3(id serial, name text UNIQUE);" -U postgres
psql prest -c "create table test4(id serial primary key, name text UNIQUE);" -U postgres
psql prest -c "create table test5(id serial, name text, celphone text);" -U postgres
psql prest -c "create table test6(nuveo serial primary key, name text, celphone text);" -U postgres
psql prest -c "insert into test (name) values ('prest tester');" -U postgres
psql prest -c "insert into test (name) values ('tester02');" -U postgres
psql prest -c "insert into test2 (name, number) values ('tester02', 2);" -U postgres
psql prest -c "insert into test3 (name) values ('prest');" -U postgres
psql prest -c "insert into test3 (name) values ('prest tester');" -U postgres
psql prest -c "insert into test5 (name, celphone) values ('prest tester', '444444');" -U postgres

# Permission tests
psql prest -c "create table test_readonly_access(id serial, name text);" -U postgres
psql prest -c "create table test_write_and_delete_access(id serial, name text);" -U postgres
psql prest -c "create table test_list_only_id(id serial, name text);" -U postgres
psql prest -c "create table test_deleteonly_access(id serial, name text);" -U postgres

psql prest -c "insert into test_readonly_access (name) values ('test01');" -U postgres
psql prest -c "insert into test_write_and_delete_access (name) values ('test01');" -U postgres
psql prest -c "insert into test_list_only_id (name) values ('test01');" -U postgres
psql prest -c "insert into test_deleteonly_access (name) values ('test01');" -U postgres


# Views
psql prest -c "create table table_to_view(id serial, name text, celphone text);" -U postgres
psql prest -c "insert into table_to_view (name, celphone) values ('gopher', '8888888')" -U postgres
psql prest -c "create view view_test as select name as player from table_to_view" -U postgres
