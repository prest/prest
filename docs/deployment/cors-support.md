---
title: "CORS Support"
date: 2017-08-30T19:06:49-03:00
weight: 6
---

[Cross-Origin Resource Sharing (CORS)](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS) is an HTTP-header based mechanism that allows a server to indicate any origins (domain, scheme, or port) other than its own from which a browser should permit loading resources. CORS also relies on a mechanism by which browsers make a "preflight" request to the server hosting the cross-origin resource, in order to check that the server will permit the actual request. In that preflight, the browser sends headers that indicate the HTTP method and headers that will be used in the actual request.

There are two settings to be made for releasing CORS (Cross-Origin Resource Sharing) in pretsd, **source** and **method**.

In the `prest.toml` you can configure the CORS allowed origin.

Example:

```toml
[cors]
alloworigin = ["https://prestd.com", "http://foo.com"]
allowheaders = ["Content-Type"]
allowmethods = ["GET", "DELETE", "POST", "PUT", "PATCH", "OPTIONS"]
```

> if you want to release all origins just use asterisk `*` as the org item, thus:
> `alloworigin = ["*"]`
