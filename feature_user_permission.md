### Table Permissions for Individual Users

Configuring individual user permissions for tables is essential in many scenarios. Consider the following example:



We have a user table with the following fields:

- `id`
- `name`
- `age`
- `salary`

The sample data is as follows:

| id   | name            | age  | salary |
| :--- | :-------------- | :--- | :----- |
| 1    | Yu Yuning       | 20   | 8.70   |
| 2    | Ren Jialun      | 24   | 850.41 |
| 3    | Zeng Lu         | 25   | 163.03 |



We have configured the permissions for this table as follows:

```toml
[[access.tables]]
name = "users"
permissions = ["read", "write", "delete"]
fields = ["id", "name", "age", "salary"]
```

This configuration allows all users to access the table. However, in practice, we might have specific requirements:

- **Staff**: Can only read the `name` and `age` fields, as `salary` is sensitive information.
- **IT personnel**: Can read and write the `id`, `name`, and `age` fields.
- **Managers**: Can access all fields and have all permissions.



The desired permission setup is as follows:

| User    | Permission  | Fields              |
| :------ | :---------- | :------------------ |
| Staff   | Read        | `name`, `age`       |
| IT      | Read, Write | `id`, `name`, `age` |
| Manager | All         | All fields          |



To achieve this, we can configure the permissions for each user type as follows:

```toml
[[access.users]]
name = "staff"
  [[access.users.tables]]
  name = "users"
  permissions = ["read"]
  fields = ["name", "age"]
```



This configuration grants the `staff` user read-only access to the `name` and `age` fields of the `users` table.

Similarly, for other user types:

```toml
[[access.users]]
name = "it"
  [[access.users.tables]]
  name = "users"
  permissions = ["read", "write"]
  fields = ["id", "name", "age"]

[[access.users]]
name = "manager"
  [[access.users.tables]]
  name = "users"
  permissions = ["read", "write", "delete"]
  fields = ["id", "name", "age", "salary"] # or use `*` to indicate all fields
```



With these configurations, we can easily achieve the desired permission setup.

Let's test the permissions:

- **Staff** (Read-only access to `name` and `age`):

  ```http
HTTP/1.1 200 OK
  Content-Type: application/json
  Vary: Origin
  Date: Mon, 10 Feb 2025 09:52:16 GMT
  Content-Length: 359
  Connection: close
  
  [
    {
      "age": 20,
      "name": "Yu Yuning"
    },
    {
      "age": 24,
      "name": "Ren Jialun"
    },
    {
      "age": 25,
      "name": "Zeng Lu"
    }
  ]
  ```
  
  

- **IT** (Read and write access to `id`, `name`, and `age`):

  ```http
HTTP/1.1 200 OK
  Content-Type: application/json
  Vary: Origin
  Date: Mon, 10 Feb 2025 09:53:42 GMT
  Content-Length: 450
  Connection: close
  
  [
    {
      "id": 1,
      "age": 20,
      "name": "Yu Yuning"
    },
    {
      "id": 2,
      "age": 24,
      "name": "Ren Jialun"
    },
    {
      "id": 3,
      "age": 25,
      "name": "Zeng Lu"
    }
  ]
  ```
  
  

- **Manager** (Full access to all fields):

  ```http
  HTTP/1.1 200 OK
  Content-Type: application/json
  Vary: Origin
  Date: Mon, 10 Feb 2025 09:55:28 GMT
  Content-Length: 628
  Connection: close
  
  [
    {
      "id": 1,
      "age": 20,
      "name": "Yu Yuning",
      "salary": 89.70
    },
    {
      "id": 2,
      "age": 24,
      "name": "Ren Jialun",
      "salary": 850.41
    },
    {
      "id": 3,
      "age": 25,
      "name": "Zeng Lu",
      "salary": 163.03
    }
  ]
  ```

If a user attempts an unauthorized operation, the system will return an error message:

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json
Vary: Origin
X-Content-Type-Options: nosniff
Date: Mon, 10 Feb 2025 09:56:19 GMT
Content-Length: 55
Connection: close

{
  "error": "{\"error\": \"authorization required\"}"
}
```



### Additional Scenarios

#### Whitelisting: Restricting Access to Managers Only

If only managers should have access to the `users` table, we can configure it as follows:

```toml
[[access.tables]]
name = "users"
permissions = []
fields = []

[[access.users]]
name = "manager"
  [[access.users.tables]]
  name = "users"
  permissions = ["read", "write", "delete"]
  fields = ["id", "name", "age", "salary"]
```



#### Blacklisting: Blocking Access for Specific Users

To block access for a specific user (e.g., a "dangerous" user), we can configure it as follows:

```toml
[[access.tables]]
name = "users"
permissions = ["read", "write", "delete"]
fields = ["id", "name", "age", "salary"]

[[access.users]]
name = "dangerous"
  [[access.users.tables]]
  name = "users"
  permissions = []
  fields = []
```

These configurations provide a flexible and secure way to manage table permissions for different user roles in our system.



### Example Configuration File:

```
[access]
restrict = true  

[[access.tables]]
name = "users"
permissions = ["read", "write", "delete"]
fields = ["id", "name", "age", "salary"]

[[access.users]]
name = "staff"
	[[access.users.tables]]
	name = "users"
	permissions = [ "read" ]
	fields = ["name", "age"]

[[access.users]]
name = "it"
	[[access.users.tables]]
	name = "users"
	permissions = [ "read", "write" ]
	fields = ["id", "name", "age"]

[[access.users]]
name = "manager"
	[[access.users.tables]]
	name = "users"
	permissions = [ "read", "write", "delete" ]
	fields = ["id", "name", "age", "salary"] # or *
```



### The full configuration file:

```

[access]
restrict = true  # can access only the tables listed below

	[[access.tables]]
	name = "Reply"
	permissions = ["read", "write", "delete"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "test"
	permissions = ["read", "write", "delete"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "testarray"
	permissions = ["read", "write", "delete"]
	fields = ["id", "data"]

	[[access.tables]]
	name = "test2"
	permissions = ["read", "write", "delete"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "test3"
	permissions = ["read", "write", "delete"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "test4"
	permissions = ["read", "write", "delete"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "test5"
	permissions = ["read", "write", "delete"]
	fields = ["*"]

	[[access.tables]]
	name = "test_readonly_access"
	permissions = ["read"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "test_write_and_delete_access"
	permissions = ["write", "delete"]

	[[access.tables]]
	name = "test_list_only_id"
	permissions = ["read"]
	fields = ["id"]

	[[access.tables]]
	name = "test6"
	permissions = ["read", "write", "delete"]
	fields = ["nuveo", "name"]

	[[access.tables]]
	name = "view_test"
	permissions = ["read"]
	fields = ["player"]

	[[access.tables]]
	name = "test_group_by_table"
	permissions = ["read"]
	fields = ["id", "name", "age", "salary"]


	# readme eg
	[[access.tables]]
	name = "users"
	permissions = ["read", "write", "delete"]
	fields = ["id", "name", "age", "salary"]

	[[access.users]]
	name = "staff"
		[[access.users.tables]]
		name = "users"
		permissions = [ "read" ]
		fields = ["name", "age"]

	[[access.users]]
	name = "it"
		[[access.users.tables]]
		name = "users"
		permissions = [ "read", "write" ]
		fields = ["id", "name", "age"]

	[[access.users]]
	name = "manager"
		[[access.users.tables]]
		name = "users"
		permissions = [ "read", "write", "delete" ]
		fields = ["id", "name", "age", "salary"] # or *



	# Table Permissions for Individual Users
	[[access.tables]]
	name = "read_table"
	permissions = ["read"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "write_table"
	permissions = ["write"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "delete_table"
	permissions = ["delete"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "read_write_table"
	permissions = ["read","write"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "read_delete_table"
	permissions = ["read","delete"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "write_delete_table"
	permissions = ["write", "delete"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "read_write_delete_table"
	permissions = ["read", "write", "delete"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "no_user_read_table"
	permissions = ["read"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "no_user_write_table"
	permissions = ["write"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "no_user_delete_table"
	permissions = ["delete"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "no_user_read_write_table"
	permissions = ["read","write"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "no_user_read_delete_table"
	permissions = ["read","delete"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "no_user_write_delete_table"
	permissions = ["write", "delete"]
	fields = ["id", "name"]

	[[access.tables]]
	name = "no_user_read_write_delete_table"
	permissions = ["read", "write", "delete"]
	fields = ["id", "name"]

	[[access.users]]
	name = "foo_read"
		[[access.users.tables]]
		name = "read_table"
		permissions = [ "read" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "write_table"
		permissions = [ "read" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "delete_table"
		permissions = [ "read" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_write_table"
		permissions = [ "read" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_delete_table"
		permissions = [ "read" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "write_delete_table"
		permissions = [ "read" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_write_delete_table"
		permissions = [ "read" ]
		fields = ["name"]

	[[access.users]]
	name = "foo_write"
		[[access.users.tables]]
		name = "read_table"
		permissions = [ "write" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "write_table"
		permissions = [ "write" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "delete_table"
		permissions = [ "write" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_write_table"
		permissions = [ "write" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_delete_table"
		permissions = [ "write" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "write_delete_table"
		permissions = [ "write" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_write_delete_table"
		permissions = [ "write" ]
		fields = ["name"]


	[[access.users]]
	name = "foo_delete"
		[[access.users.tables]]
		name = "read_table"
		permissions = [ "delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "write_table"
		permissions = [ "delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "delete_table"
		permissions = [ "delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_write_table"
		permissions = [ "delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_delete_table"
		permissions = [ "delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "write_delete_table"
		permissions = [ "delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_write_delete_table"
		permissions = [ "delete" ]
		fields = ["name"]

	[[access.users]]
	name = "foo_read_write"
		[[access.users.tables]]
		name = "read_table"
		permissions = [ "read","write" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "write_table"
		permissions = [ "read","write" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "delete_table"
		permissions = [ "read","write" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_write_table"
		permissions = [ "read","write" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_delete_table"
		permissions = [ "read","write" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "write_delete_table"
		permissions = [ "read","write" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_write_delete_table"
		permissions = [ "read","write" ]
		fields = ["name"]



	[[access.users]]
	name = "foo_read_delete"
		[[access.users.tables]]
		name = "read_table"
		permissions = [ "read","delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "write_table"
		permissions = [ "read","delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "delete_table"
		permissions = [ "read","delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_write_table"
		permissions = [ "read","delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_delete_table"
		permissions = [ "read","delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "write_delete_table"
		permissions = [ "read","delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_write_delete_table"
		permissions = [ "read","delete" ]
		fields = ["name"]



	[[access.users]]
	name = "foo_write_delete"
		[[access.users.tables]]
		name = "read_table"
		permissions = [ "write","delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "write_table"
		permissions = [ "write","delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "delete_table"
		permissions = [ "write","delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_write_table"
		permissions = [ "write","delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_delete_table"
		permissions = [ "write","delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "write_delete_table"
		permissions = [ "write","delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_write_delete_table"
		permissions = [ "write","delete" ]
		fields = ["name"]

	[[access.users]]
	name = "foo_read_write_delete"
		[[access.users.tables]]
		name = "read_table"
		permissions = [ "read", "write","delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "write_table"
		permissions = [ "read", "write","delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "delete_table"
		permissions = [ "read", "write","delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_write_table"
		permissions = [ "read", "write","delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_delete_table"
		permissions = [ "read", "write","delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "write_delete_table"
		permissions = [ "read", "write","delete" ]
		fields = ["name"]

		[[access.users.tables]]
		name = "read_write_delete_table"
		permissions = [ "read", "write","delete" ]
		fields = ["name"]

```

