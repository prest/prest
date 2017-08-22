# Negroni example

This is an example of how to use the Negroni middleware.

# Using it

To try this out, first install all dependencies with `go install` and then run `go run main.go` to start the app.

* Call `http://localhost:3001/ping` to get a JSon response without the need of a JWT.
* Call `http://localhost:3001/secured/ping` with a JWT signed with `My Secret` to get a response back.