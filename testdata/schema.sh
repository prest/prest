DB_HOST=${PREST_PG_HOST:-localhost}
DB_USER=${PREST_PG_USER:-postgres}
DB_PORT=${PREST_PG_PORT:-5438}
DB_NAME=${PREST_PG_DATABASE:-prest} 

psql -h $DB_HOST -p $DB_PORT -U $DB_USER -c "DROP DATABASE IF EXISTS $DB_NAME"
psql -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create database $DB_NAME;"

# Create tables
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create table test(id serial, name text);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create table test2(name text, number integer);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create table test3(id serial, name text UNIQUE);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create table test4(id serial primary key, name text UNIQUE);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create table test5(id serial, name text, celphone text);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create table test6(nuveo serial primary key, name text, celphone text);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create table test7(id serial, name text, surname text);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create table test8(nameforjoin text);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create table test_readonly_access(id serial, name text);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create table test_write_and_delete_access(id serial, name text);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create table test_list_only_id(id serial, name text);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create table test_deleteonly_access(id serial, name text);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create table test_jsonb_bug(id serial NOT NULL, name text, data jsonb);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create table testjson(name text, data jsonb);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create table testarray(id serial, data character varying(250)[]);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create table test_empty_table(id serial, data character varying(250)[]);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create table test_group_by_table(id serial, name text, age integer, salary int);"

# Inserts
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into test (name) values ('prest tester');"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into test (name) values ('tester02');"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into test2 (name, number) values ('tester02', 2);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into test3 (name) values ('prest');"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into test3 (name) values ('prest tester');"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into test5 (name, celphone) values ('prest tester', '444444');"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into test7 (name, surname) values ('gopher', 'da silva');"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into test_readonly_access (name) values ('test01');"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into test_write_and_delete_access (name) values ('test01');"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into test_list_only_id (name) values ('test01');"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into test_deleteonly_access (name) values ('test01');"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into test_jsonb_bug (name, data) values('goku', '{\"techniques\": [\"kamehameha\", \"kaioken\", \"genki-dama\"]}');"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into test_jsonb_bug (name, data) values('goku', '{\"description\": \"testing\"}');"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into testarray (id, data) values(100, '{\"Gohan\",\"Goten\"}');"

psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into test_group_by_table(name, age, salary) values('joao', 20, 1250);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into test_group_by_table(name, age, salary) values('maria', 19, 3999);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into test_group_by_table(name, age, salary) values('gopher', 20, 100);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into test_group_by_table(name, age, salary) values('guitarra humana', 19, 3998);"

# Views
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create table table_to_view(id serial, name text, celphone text);"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "insert into table_to_view (name, celphone) values ('gopher', '8888888')"
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create view view_test as select name as player from table_to_view"