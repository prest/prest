---
title: "Development Guides"
date: 2021-09-02T13:39:24-03:00
weight: 6
---

_**prestd**_ is written in the [go language](https://golang.org) and we use the best practices recommended by the language itself to simplify its contribution.
If you are not familiar with the language, read the [Effective Go](https://golang.org/doc/effective_go).

## Development usage

As mentioned before prest is written in **go**, as it is in the document topic of using prest in development mode it is important to know the go language path structure, if you don't know it read the page [How to Write Go Code (with GOPATH)](https://golang.org/doc/gopath_code).

> Assuming you do not have the [repository cloned](https://github.com/prest/prest "git clone git@github.com:prest/prest.git") locally, we are assuming you are reading this page for the first time

Download all of pREST's dependencies

```sh
git clone git@github.com:prest/prest.git && cd prest
go mod download
```

We recommend using `go run` for development environment, remember that it is necessary environment variables for _p_**REST** to connect to PostgreSQL - we will explain in the next steps how to do it

```sh
go run cmd/prestd/main.go
```

Building a **local version** (we will not use flags for production environment)

```sh
go build -o prestd cmd/prestd/main.go
```

Executing the `prestd` after generating binary or using `go run`

```sh
PREST_PG_USER=postgres PREST_PG_PASS=postgres PREST_PG_DATABASE=prest PREST_PG_PORT=5432 PREST_HTTP_PORT=3010 ./prestd
```

> to use `go run` replace `./prestd` with `go run`

or use `'prest.toml'` file as a preset configuration, insert a user to see the changes

## Dev Container

A [devcontainer](https://code.visualstudio.com/docs/remote/containers) is used by the VS Code Remote Containers extension and works by creating a Docker container to do your development in.

> Usually preparing the development environment is not a simple job, especially when we are talking about software that depends on other software for its operation, **this is where [devcontainer](https://code.visualstudio.com/docs/remote/containers) come in**.

As the development environment is within Docker, you supply the [`Dockerfile`](https://docs.docker.com/engine/reference/builder/) and VS Code will take care of building the image and starting the container for you. Then since you control the `Dockerfile` you can have it install any software you need for your project, set the right version of Node, install global packages, etc.

This is just a plain old `Dockerfile`, you can run it without VS Code using the standard Docker tools and mount a volume in, but the power comes when you combine it with the [`devcontainers.json`](https://code.visualstudio.com/docs/remote/devcontainerjson-reference) file, which gives VS Code instructions on how to configure itself.

Using golang + prettier? Tell the devcontainer to install those extensions so the user has them already installed. Want some VS Code settings enabled by default, specify them so users don’t have to know about it.

### GitHub Codespaces

A codespace is a development environment that's hosted in the cloud. You can customize your project for Codespaces by committing configuration files to your repository (often known as Configuration-as-Code), which creates a repeatable codespace configuration for all users of your project.

Codespaces run on a variety of VM-based compute options hosted by GitHub.com, which you can configure from 2 core machines up to 32 core machines. You can connect to your codespaces from the browser or locally using Visual Studio Code.

![GitHub codespaces diagram](https://docs.github.com/assets/cb-49622/images/help/codespaces/codespaces-diagram.png)

#### How to use the prestd in Codespaces

1. Access the address to create prestd codespace [here](https://github.com/prest/prest/codespaces)
2. Select the **branch** (we recommend using `main`) and click **create codespace**
  ![codespace: screenshot of select branch and create codespace button to click](/assets/setup/development-guide/codespace-step-2.png)
3. wait for the setup... it may take a few minutes
  ![codespace: screenshot of codespace build process](/assets/setup/development-guide/codespace-step-3.png)

**Done** (_#congrats_), you have a development environment for `prestd` with **PostgreSQL** (configured and integrated with `prestd`), vscode plugins (for golang), database viewer, etc.

![codespace: screenshot of prestd development environment using codespace](/assets/setup/development-guide/codespace-step-4.png)

**Database viewer:**

![codespace: screenshot of prestd development environment using codespace](/assets/setup/development-guide/codespace-database-viewer.png)

## Next step in development

If you have come this far I assume that your development environment is working, right?

> if not, go back to the previous topics

To get the environment working with _"all the right stuff"_ we recommend setting up (and activating) the API authentication system. To do this, follow the steps below:

> we are writing the example using go code (not binario or docker, if you want to run the commands via docker [see here](/prestd/#test-using-docker))

```sh
# Run data migration to create user structure for access (JWT)
go run cmd/prestd/main.go migrate up auth

# Create user and password for API access (via JWT)
## user: prest
## pass: prest
psql -d prest -U prest -h localhost -c "INSERT INTO prest_users (name, username, password) VALUES ('pREST Full Name', 'prest', MD5('prest'))"
# Check if the user was created successfully (by doing a select on the table)
psql -d prest -U prest -h localhost -c "select * from prest_users"
```

**Now the fun begins:**

```sh
# Run prestd server
go run cmd/prestd/main.go
# Generate JWT Token with user and password created
curl -i -X POST http://127.0.0.1:3000/auth -H "Content-Type: application/json" -d '{"username": "prest", "password": "prest"}'
# Access endpoint using JWT Token
curl -i -X GET http://127.0.0.1:3000/prest/public/prest_users -H "Accept: application/json" -H "Authorization: Bearer {TOKEN}"
```

## Execute unit tests locally (integration/e2e)

pREST's unit tests depend on a working Postgres database for SQL query execution, to simplify the preparation of the local environment we use docker (and docker-compose) to upload the environment with Postgres.

**all tests:**

```sh
docker-compose -f testdata/docker-compose.yml up --abort-on-container-exit
```

**package-specific testing:**
_in the example below the `config` package will be tested_

```sh
docker-compose -f testdata/docker-compose.yml run --rm prest-test sh ./testdata/runtest.sh ./config
```

**specific function test:**
_in the example below will run the test `TestGetDefaultPrestConf` from the `config` package, don't forget to call the `TestMain` function before your function_

```sh
docker-compose -f testdata/docker-compose.yml run prest-test sh ./testdata/runtest.sh ./config -run TestMain,TestGetDefaultPrestConf
```

## Version - Patterns

_**prestd**_ has the `main` branch as a tip branch and has version branches such as `v1.1`. `v1.1` is a release branch and we will tag `v1.1.0` for binary download. If `v1.1.0` has bugs, we will accept pull requests on the `v1.1` branch and publish a `v1.1.1` tag, after bringing the bug fix also to the main branch.

Since the `main` branch is a tip version, if you wish to use pREST in production, please download the latest release tag version. All the branches will be protected via GitHub, all the PRs to every branch must be reviewed by two maintainers and must pass the automatic tests.
