---
title: "CORS Support"
date: 2017-08-30T19:06:49-03:00
weight: 14
menu: main
---

[Cross-Origin Resource Sharing (CORS)](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS) is an HTTP-header based mechanism that allows a server to indicate any origins (domain, scheme, or port) other than its own from which a browser should permit loading resources. CORS also relies on a mechanism by which browsers make a "preflight" request to the server hosting the cross-origin resource, in order to check that the server will permit the actual request. In that preflight, the browser sends headers that indicate the HTTP method and headers that will be used in the actual request.

There are two settings to be made for releasing CROS (Cross-Origin Resource Sharing) in pREST, **source** and **method**.

In the `prest.toml` you can configurate the CORS allowed origin.

Example:

```toml
[cors]
alloworigin = ["https://prestd.com", "http://foo.com"]
allowheaders = ["GET", "DELETE", "POST", "PUT", "PATCH", "OPTIONS"]
```
