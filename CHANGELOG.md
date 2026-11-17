# Changelog

What each release changed for you, newest first. Each line is a commit's summary, linked to its full description and diff.

## 1.0.4 - 2026-11-17

### Features

- Require internetdata v2.14.0: cap_reached on a paid answer past its overage cap ([`6f72b57`](https://github.com/internetdata/sdk-go-nethttp/commit/6f72b57fa251d60169eed70885de5722ca67910c))

## 1.0.3 - 2026-11-16

### Features

- Require internetdata v2.13.0: an AS's rank, neighbors and hosted domains ([`fc2dd6d`](https://github.com/internetdata/sdk-go-nethttp/commit/fc2dd6d7c7c8d89b0bc04c601e2673e4f42e3a65))

## 1.0.2 - 2026-11-15

### Features

- Require internetdata v2.12.0: hosted_domains lists its 10 most popular domains ([`1a76e0a`](https://github.com/internetdata/sdk-go-nethttp/commit/1a76e0a40332f7168c932fee93a9b831ee931f91))

## 1.0.1 - 2026-11-14

### Fixes

- Require internetdata v2.11.1: a 403 can mean the key lacks the ip.lookup scope ([`fbbd6cb`](https://github.com/internetdata/sdk-go-nethttp/commit/fbbd6cb6217c6d74a3f9798414825f34bb2abe45))

## 1.0.0 - 2026-11-14

### Features

- Attach the visitor's InternetData record to every request's context, read with `FromContext`: the location, the network and the company behind the address, and the anonymity and hosting flags your plan serves ([`61ef6e7`](https://github.com/internetdata/sdk-go-nethttp/commit/61ef6e785dd05a9e4c196f1cbad2a63ac39bde49))
- Refuse a request with a `BlockCondition` written in the shape of a result, by country, network type, anonymity or any other member, with 403 or an `OnBlocked` of your own ([`61ef6e7`](https://github.com/internetdata/sdk-go-nethttp/commit/61ef6e785dd05a9e4c196f1cbad2a63ac39bde49))
- Choose where the client address comes from: the socket peer by default, `XFFIPSelector`, `HeaderIPSelector` for a header your edge writes, or a function of your own ([`61ef6e7`](https://github.com/internetdata/sdk-go-nethttp/commit/61ef6e785dd05a9e4c196f1cbad2a63ac39bde49))
- Fail open on any lookup failure unless `FailClosed` is set, with a 2500 ms bound, no retry and a cache per middleware by default ([`61ef6e7`](https://github.com/internetdata/sdk-go-nethttp/commit/61ef6e785dd05a9e4c196f1cbad2a63ac39bde49))
