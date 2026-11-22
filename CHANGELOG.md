# Changelog

What each release changed for you, newest first. Each line is a commit's summary, linked to its full description and diff.

## 1.0.10 - 2026-11-22

### Features

- Require internetdata v2.16.0, which reads is_bot and the bot row ([`fbe72ce`](https://github.com/internetdata/sdk-go-nethttp/commit/fbe72ced52b76743ae0d8441ba0af6ffd514c4ca))
- README: a condition on is_bot, which works on every plan ([`55be069`](https://github.com/internetdata/sdk-go-nethttp/commit/55be069894e69bb42158f9e8768129a369c553f2))

## 1.0.9 - 2026-11-20

### Fixes

- Say rate limit where the docs named a per-minute one ([`cb15e80`](https://github.com/internetdata/sdk-go-nethttp/commit/cb15e803e1257b78ea27b942cafc4043bd0a23bd))

## 1.0.8 - 2026-11-20

### Fixes

- Require internetdata v2.15.0: a paid domain answer's hosting, co-hosted domains, home page, redirects and backlinks ([`afaab20`](https://github.com/internetdata/sdk-go-nethttp/commit/afaab20fc63c19d0071f434d1f23ff67a105f825))

## 1.0.7 - 2026-11-20

### Fixes

- Require internetdata v2.14.3: the spec re-pinned to 2026.11.20 ([`e7f4e8e`](https://github.com/internetdata/sdk-go-nethttp/commit/e7f4e8e556b2e765f9bc39e89f738ff0b9ff9b99))

## 1.0.6 - 2026-11-19

### Fixes

- Require internetdata v2.14.2: a reduced answer is cached a minute at most ([`206586c`](https://github.com/internetdata/sdk-go-nethttp/commit/206586c65819edd588760639369d8e87c0858945))

## 1.0.5 - 2026-11-18

### Fixes

- Require internetdata v2.14.1: the spec as republished ([`864ede8`](https://github.com/internetdata/sdk-go-nethttp/commit/864ede80a19b304fbb4785bf7c0007cfef87bcc2))

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
