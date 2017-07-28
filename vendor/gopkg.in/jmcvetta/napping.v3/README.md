# Napping: HTTP for Gophers

Package `napping` is a [Go][] client library for interacting with
[RESTful APIs][].  Napping was inspired  by Python's excellent [Requests][]
library.


## Status

| System    | Status                                                                                                                |
|-----------|:---------------------------------------------------------------------------------------------------------------------:|
| Travis CI | [![Travis Build Status](https://travis-ci.org/jmcvetta/napping.png)](https://travis-ci.org/jmcvetta/napping)          |
| CircleCI  | [![Circle CI](https://circleci.com/gh/jmcvetta/napping.svg?style=svg)](https://circleci.com/gh/jmcvetta/napping)      |
| Coveralls | [![Coveralls](https://img.shields.io/coveralls/jmcvetta/napping/master.svg)](https://coveralls.io/r/jmcvetta/napping) |
| Codecov   | [![Codecov](https://img.shields.io/codecov/c/github/jmcvetta/napping.svg)](https://codecov.io/gh/jmcvetta/napping)    |

Used by, and developed in conjunction with, [Neoism][].


## Installation 

### Requirements

Napping is [tested with Go 1.3 or later](https://github.com/jmcvetta/napping/blob/develop/.travis.yml#L2).


### Development

```
go get github.com/jmcvetta/napping
```

### Stable

Napping is versioned using [`gopkg.in`](http://gopkg.in).  

Current release is `v3`.

```
go get gopkg.in/jmcvetta/napping.v3
```


## Documentation

See [![GoDoc](http://godoc.org/github.com/jmcvetta/napping?status.png)](http://godoc.org/github.com/jmcvetta/napping)
for automatically generated API documentation.

Check out [github_auth_token][auth-token] for a working example
showing how to retrieve an auth token from the Github API.


## Support

Support and consulting services are available from [Silicon Beach Heavy
Industries](http://siliconheavy.com).



## Contributing

Contributions in the form of Pull Requests are gladly accepted.  Before
submitting a PR, please ensure your code passes all tests, and that your
changes do not decrease test coverage.  I.e. if you add new features also add
corresponding new tests.


## License

This is Free Software, released under the terms of the [GPL v3][].


[Go]:           http://golang.org
[RESTful APIs]: http://en.wikipedia.org/wiki/Representational_state_transfer#RESTful_web_APIs
[Requests]:     http://python-requests.org
[GPL v3]:       http://www.gnu.org/copyleft/gpl.html
[auth-token]:   https://github.com/jmcvetta/napping/blob/master/examples/github_auth_token/github_auth_token.go
[Neoism]:       https://github.com/jmcvetta/neoism
