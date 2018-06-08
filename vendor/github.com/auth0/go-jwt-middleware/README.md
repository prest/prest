# GO JWT Middleware

A middleware that will check that a [JWT](http://jwt.io/) is sent on the `Authorization` header and will then set the content of the JWT into the `user` variable of the request.

This module lets you authenticate HTTP requests using JWT tokens in your Go Programming Language applications. JWTs are typically used to protect API endpoints, and are often issued using OpenID Connect.

## Key Features

* Ability to **check the `Authorization` header for a JWT**
* **Decode the JWT** and set the content of it to the request context

## Installing

````bash
go get github.com/auth0/go-jwt-middleware
````

## Using it

You can use `jwtmiddleware` with default `net/http` as follows.

````go
// main.go
package main

import (
  "fmt"
  "net/http"

  "github.com/auth0/go-jwt-middleware"
  "github.com/dgrijalva/jwt-go"
  "github.com/gorilla/context"
)

var myHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  user := context.Get(r, "user")
  fmt.Fprintf(w, "This is an authenticated request")
  fmt.Fprintf(w, "Claim content:\n")
  for k, v := range user.(*jwt.Token).Claims {
    fmt.Fprintf(w, "%s :\t%#v\n", k, v)
  }
})

func main() {
  jwtMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
    ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
      return []byte("My Secret"), nil
    },
    // When set, the middleware verifies that tokens are signed with the specific signing algorithm
    // If the signing method is not constant the ValidationKeyGetter callback can be used to implement additional checks
    // Important to avoid security issues described here: https://auth0.com/blog/2015/03/31/critical-vulnerabilities-in-json-web-token-libraries/
    SigningMethod: jwt.SigningMethodHS256,
  })

  app := jwtMiddleware.Handler(myHandler)
  http.ListenAndServe("0.0.0.0:3000", app)
}
````

You can also use it with Negroni as follows:

````go
// main.go
package main

import (
  "context"
  "fmt"
  "net/http"

  "github.com/auth0/go-jwt-middleware"
  "github.com/codegangsta/negroni"
  "github.com/dgrijalva/jwt-go"
  "github.com/gorilla/mux"
)

var myHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  user := r.Context().Value("user");
  fmt.Fprintf(w, "This is an authenticated request")
  fmt.Fprintf(w, "Claim content:\n")
  for k, v := range user.(*jwt.Token).Claims {
    fmt.Fprintf(w, "%s :\t%#v\n", k, v)
  }
})

func main() {
  r := mux.NewRouter()

  jwtMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
    ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
      return []byte("My Secret"), nil
    },
    // When set, the middleware verifies that tokens are signed with the specific signing algorithm
    // If the signing method is not constant the ValidationKeyGetter callback can be used to implement additional checks
    // Important to avoid security issues described here: https://auth0.com/blog/2015/03/31/critical-vulnerabilities-in-json-web-token-libraries/
    SigningMethod: jwt.SigningMethodHS256,
  })

  r.Handle("/ping", negroni.New(
    negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
    negroni.Wrap(myHandler),
  ))
  http.Handle("/", r)
  http.ListenAndServe(":3001", nil)
}
````

## Options

````go
type Options struct {
  // The function that will return the Key to validate the JWT.
  // It can be either a shared secret or a public key.
  // Default value: nil
  ValidationKeyGetter jwt.Keyfunc
  // The name of the property in the request where the user information
  // from the JWT will be stored.
  // Default value: "user"
  UserProperty string
  // The function that will be called when there's an error validating the token
  // Default value: https://github.com/auth0/go-jwt-middleware/blob/master/jwtmiddleware.go#L35
  ErrorHandler errorHandler
  // A boolean indicating if the credentials are required or not
  // Default value: false
  CredentialsOptional bool
  // A function that extracts the token from the request
  // Default: FromAuthHeader (i.e., from Authorization header as bearer token)
  Extractor TokenExtractor
  // Debug flag turns on debugging output
  // Default: false  
  Debug bool
  // When set, all requests with the OPTIONS method will use authentication
  // Default: false
  EnableAuthOnOptions bool,
  // When set, the middelware verifies that tokens are signed with the specific signing algorithm
  // If the signing method is not constant the ValidationKeyGetter callback can be used to implement additional checks
  // Important to avoid security issues described here: https://auth0.com/blog/2015/03/31/critical-vulnerabilities-in-json-web-token-libraries/
  // Default: nil
  SigningMethod jwt.SigningMethod
}
````

### Token Extraction

The default value for the `Extractor` option is the `FromAuthHeader`
function which assumes that the JWT will be provided as a bearer token
in an `Authorization` header, i.e.,

```
Authorization: bearer {token}
```

To extract the token from a query string parameter, you can use the
`FromParameter` function, e.g.,

```go
jwtmiddleware.New(jwtmiddleware.Options{
  Extractor: jwtmiddleware.FromParameter("auth_code"),
})
```

In this case, the `FromParameter` function will look for a JWT in the
`auth_code` query parameter.

Or, if you want to allow both, you can use the `FromFirst` function to
try and extract the token first in one way and then in one or more
other ways, e.g.,

```go
jwtmiddleware.New(jwtmiddleware.Options{
  Extractor: jwtmiddleware.FromFirst(jwtmiddleware.FromAuthHeader,
                                     jwtmiddleware.FromParameter("auth_code")),
})
```

## Examples

You can check out working examples in the [examples folder](https://github.com/auth0/go-jwt-middleware/tree/master/examples)


## What is Auth0?

Auth0 helps you to:

* Add authentication with [multiple authentication sources](https://docs.auth0.com/identityproviders), either social like **Google, Facebook, Microsoft Account, LinkedIn, GitHub, Twitter, Box, Salesforce, amont others**, or enterprise identity systems like **Windows Azure AD, Google Apps, Active Directory, ADFS or any SAML Identity Provider**.
* Add authentication through more traditional **[username/password databases](https://docs.auth0.com/mysql-connection-tutorial)**.
* Add support for **[linking different user accounts](https://docs.auth0.com/link-accounts)** with the same user.
* Support for generating signed [Json Web Tokens](https://docs.auth0.com/jwt) to call your APIs and **flow the user identity** securely.
* Analytics of how, when and where users are logging in.
* Pull data from other sources and add it to the user profile, through [JavaScript rules](https://docs.auth0.com/rules).

## Create a free Auth0 Account

1. Go to [Auth0](https://auth0.com) and click Sign Up.
2. Use Google, GitHub or Microsoft Account to login.

## Issue Reporting

If you have found a bug or if you have a feature request, please report them at this repository issues section. Please do not report security vulnerabilities on the public GitHub issue tracker. The [Responsible Disclosure Program](https://auth0.com/whitehat) details the procedure for disclosing security issues.

## Author

[Auth0](auth0.com)

## License

This project is licensed under the MIT license. See the [LICENSE](LICENSE.txt) file for more info.
