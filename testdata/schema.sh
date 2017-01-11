psql -c "create database prest;" -U postgres
psql prest -c "create table test(id serial, name text);" -U postgres
psql prest -c "create table test2(name text, number integer);" -U postgres
psql prest -c "create table test3(id serial, name text UNIQUE);" -U postgres
psql prest -c "create table test4(id serial, name text UNIQUE);" -U postgres
psql prest -c "create table test5(id serial, name text, celphone text);" -U postgres
psql prest -c "insert into test (name) values ('prest tester');" -U postgres
psql prest -c "insert into test (name) values ('tester02');" -U postgres
psql prest -c "insert into test2 (name, number) values ('tester02', 2);" -U postgres
psql prest -c "insert into test3 (name) values ('prest');" -U postgres
psql prest -c "insert into test3 (name) values ('prest tester');" -U postgres
psql prest -c "insert into test5 (name, celphone) values ('prest tester', '444444');" -U postgres
