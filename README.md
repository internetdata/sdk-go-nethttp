# [<img src="https://s3.internetdata.io/internetdata-public/brand/mark.svg" alt="InternetData" height="28"/>](https://internetdata.io/) InternetData net/http Middleware

[![Go Reference](https://pkg.go.dev/badge/github.com/internetdata/sdk-go-nethttp.svg)](https://pkg.go.dev/github.com/internetdata/sdk-go-nethttp)
[![license](https://img.shields.io/github/license/internetdata/sdk-go-nethttp.svg)](LICENSE)

The official `net/http` middleware for the [InternetData](https://internetdata.io) API.

It looks up the visitor behind each request, where they are, which network they come from and whether it's a VPN, a proxy, Tor or a hosting server, and puts the answer on the request context. Blocking is opt-in.

Anything that takes a `func(http.Handler) http.Handler` works with this: the standard library, [chi](https://github.com/go-chi/chi), [gorilla/mux](https://github.com/gorilla/mux), and echo through `echo.WrapMiddleware`. Gin has its own handler type and its own package, [sdk-go-gin](https://github.com/internetdata/sdk-go-gin).

## Getting Started

```bash
go get github.com/internetdata/sdk-go-nethttp
```

Requires Go 1.26 or newer.

Without an API key a lookup is on Unlimited, the free plan: the country, the network and the company behind the address, Tor, relay, anycast, carrier and satellite space, bots and the service flags, with no daily or monthly cap. Every lookup your app makes comes from your server's one address, though, so a busy site will see bursts refused. A paid plan answers every field, VPN, the proxy flags, hosting, CDN and the city among them; create a key in the [console](https://app.internetdata.io). A free account tries every field.

```go
package main

import (
    "fmt"
    "net/http"
    "os"

    internetdatahttp "github.com/internetdata/sdk-go-nethttp"
    "github.com/internetdata/sdk-go/v2/middleware"
)

func main() {
    guard := internetdatahttp.Must(internetdatahttp.Options{
        Options: middleware.Options[*http.Request]{
            APIKey: os.Getenv("INTERNETDATA_API_KEY"),
        },
    })

    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        lookup := internetdatahttp.FromContext(r.Context())
        if lookup.Result != nil {
            fmt.Fprintf(w, "Hello, visitor from %s\n", lookup.Result.Location.CountryName)
            return
        }
        fmt.Fprintln(w, "Hello")
    })

    http.ListenAndServe(":8080", guard(mux))
}
```

By default nothing is blocked. Every request gets a `Lookup` on its context and your own code decides what it means: a currency to show, a sign-up to review, a page to keep from some countries.

`Must` panics on a misconfiguration, which suits a package-level variable. `New` returns the error instead.

## Blocking

Set a `BlockCondition` and a matching request is answered with `403` and never reaches your handlers.

```go
Options: middleware.Options[*http.Request]{
    APIKey: os.Getenv("INTERNETDATA_API_KEY"),
    BlockCondition: middleware.Conditions{
        {"location": middleware.BlockCondition{"country": internetdatahttp.AnyOf("CU", "IR", "KP")}},
    },
},
```

A condition is written in the shape of a result, keyed by the same names the API uses, and only the members you name are considered, so it can refuse on anything in the answer. The country works on every plan, and so do the first three of these:

```go
// the network's type
middleware.Conditions{{"asn": middleware.BlockCondition{"type": "hosting"}}}

// anycast and satellite space: a list is OR
middleware.Conditions{{"is_anycast": true}, {"is_satellite": true}}

// crawlers, monitors and scanners
middleware.Conditions{{"is_bot": true}}

// traffic from servers, not people
middleware.Conditions{{"is_hosting": true}, {"is_cdn": true}}

// anonymity
middleware.Conditions{{"is_vpn": true}, {"is_tor": true}, {"is_resproxy": true}}

// one provider
middleware.Conditions{{"is_vpn": true, "vpn": middleware.BlockCondition{"provider": "nordvpn"}}}

// a numeric threshold
middleware.Conditions{{"resproxy": middleware.BlockCondition{"hits": internetdatahttp.Gte(5)}}}
```

Values are matched by equality, strings without regard to case. `AnyOf` means any-of. `Gte`, `Gt`, `Lte` and `Lt` compare numbers and chain into a range (`Gte(5).Lt(100)`); every bound you give must hold. Against a member the answer holds as a list (`web`'s rows, `pubproxy.protocols`), a value matches when any one element does. Members set to `false` or `nil` are ignored, so a condition states the signals you act on; one that constrains nothing would match every request, and is refused when the middleware is built rather than silently blocking all your traffic.

Replace the refusal with `OnBlocked`:

```go
OnBlocked: func(w http.ResponseWriter, r *http.Request, lookup *middleware.Lookup) {
    http.Error(w, "Not available in your country", http.StatusUnavailableForLegalReasons)
},
```

## Where the client address comes from

This is the setting that decides whether any of the above works, and it is the one thing only you can get right.

By default the middleware uses `r.RemoteAddr`, with the port stripped. **`net/http` has no trusted-proxy setting**, so that is the socket peer and nothing else. If your app sits behind nginx, a load balancer, or a CDN, every visitor arrives wearing your proxy's address, and that is the address that gets looked up: every visitor gets your proxy's country and network, and if it is a hosting address, a hosting rule blocks everyone. nginx on the same machine is `127.0.0.1` and a load balancer usually has a private address, which the middleware answers locally without a lookup, as an address with no country, and logs a warning.

For an edge that writes the address into its own header, name the header:

```go
IPSelector: internetdatahttp.HeaderIPSelector("CF-Connecting-IP"),  // or True-Client-IP
```

`XFFIPSelector(0)` reads the left-most `X-Forwarded-For` entry. Be aware that the left-most entry is whatever the caller sent, because proxies append to that header, so it's only trustworthy when an edge you control overwrites it. If you know how many proxies sit in front, count from the right instead: `XFFIPSelector(1)` is the address your nearest proxy saw.

Anything else, pass your own function. It receives the request and returns an address:

```go
IPSelector: func(r *http.Request) string { return r.Header.Get("X-Real-IP") },
```

If the address resolves to a private one, the middleware says so once through its logger. That is expected on localhost and is the signal to fix your configuration anywhere else.

## When a lookup fails

The request is let through, and the reason is on `Lookup.Err`. Our outage should not become yours, so a network failure, a rate limit or a rejected key all fail open.

```go
lookup := internetdatahttp.FromContext(r.Context())
if lookup.Err != nil {
    slog.Warn("internetdata unavailable", "err", lookup.Err)
}
```

Set `FailClosed: true` to block instead. Private addresses are answered locally and never fail, so this will not lock you out in development.

## Cost and latency

Answers are cached per middleware for an hour, so a returning visitor costs nothing, and private addresses never leave the process. A cache miss is one request to our API, bounded at 2500 ms by default and not retried: on a request path, failing open quickly beats holding a visitor while we try again. Change either with `Timeout` and `Retries` in the options, not on a client: the middleware sets both on every lookup it makes. The cache is sized on a client you build yourself (`internetdata.WithCache`) and pass as `Client`.

Wrap the routes that matter rather than the whole server, or skip what you do not care about:

```go
Skip: func(r *http.Request) bool { return strings.HasPrefix(r.URL.Path, "/static") },
```

If you already hold an `internetdata.Client`, pass it as `Client` and the middleware will share it rather than building a second cache.

Beyond a few million distinct visitors a day, stop calling the API per request: look addresses up locally in [our databases](https://internetdata.io/databases) instead.

## Absent is not false

Unlimited, with a key or without one, leaves out `IsVpn`, `IsHosting`, `IsCdn` and the four proxy flags, the detail behind them, and the paid members inside `Location`, `Asn`, `Company` and `Abuse`, such as `Location.City`. A field your plan does not include is a nil pointer, which means "not in your plan" rather than "checked, and no".

```go
internetdata.BoolValue(lookup.Result.IsVpn)  // when you only want the flag
```

A `BlockCondition` naming a member your plan does not serve, `location.city` on Unlimited included, can never match, so the middleware warns once instead of failing silently. Set `OnMissingField: middleware.MissingFieldError` to make it an error.

## Other Libraries

There are official InternetData client libraries available for many languages including PHP, Python, Go, Java, and Ruby. See our GitHub at https://github.com/internetdata for more.

## About InternetData

Geolocation, anonymity, ownership, network, service and domain data for any IP address, AS or domain, by API or as databases you download.

[<picture><source media="(prefers-color-scheme: dark)" srcset="https://s3.internetdata.io/internetdata-public/brand/logo-dark.svg"><img src="https://s3.internetdata.io/internetdata-public/brand/logo.svg" alt="InternetData" height="40"/></picture>](https://internetdata.io/)

## License

This project is licensed under the [MIT License](LICENSE).
