# Swagger UI demo

This tiny server serves the static files from the `html/` folder and exposes a Swagger UI interface from the `swagger.json` file.

## Getting started

From the project root, run:

```bash
go run ./testdata/www
```

## Access links

Once the server is running, open:

- http://localhost:8080/ : default static page
- http://localhost:8080/swagger : Swagger UI interface
- http://localhost:8080/swagger.json : OpenAPI/Swagger JSON document

## Structure

- `html/` : static files served by the server
- `swagger.json` : OpenAPI/Swagger specification
- `main.go` : Go HTTP server

## Notes

- The server listens on port `8080`.
- To change the port, update the value in `main.go`.
