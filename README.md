# Promoter

Promoter allows users to pay for premium access to a Skynet portal with various cryptocurrencies. The actual payments
are processed by specialized payment processors
e.g. [siacoin-promoter](https://github.com/SkynetLabs/siacoin-promoter). Promoter converts the fund sent by the user
into credits and allows the user to use their credit balance to pay for specific packages.

## Status: Unfinished

This project is unfinished and it's not ready for use, even in testing.

## Architecture

### Purpose

Skynet portal operators need to be able to monetize their portals. While each operator is free to decide how they want
to approach this, we try to provide them with some basic tools to help them along the way.

We have two tools that enable portal operators to collect subscription payments:

- Stripe, which supports fiat payments
- Promoter, which supports crypto payments

In order to choose between them you can use the `ACCOUNTS_PROMOTER` environment variable which has two valid values -
`stripe` (default) and `promoter`.

*It's important to note that a portal can only use one of those payment systems. If you switch from one to the other
there is no guarantee that your portal will continue to function correctly - users might fail to be promoted or demoted,
their tier in the previously used system will not be respected by the new one. This might change in the future if our
Stripe integration is reworked as a payment processor for Promoter but there are no current plans for that.*

### Responsibilities

Promoter is responsible for tracking the current credit balance of each user and for handling promotion and demotion
events. Those occur when a user requests to subscribe to a certain tier of access for a given period and, respectively,
when either that period expires or the user decides to cancel their subscription.

#### Credits

Promoter is a currency-agnostic service which tracks the balances of users in abstract units called "credits". The
prices of tiers of access are defined in credits by the portal operator. The price of a credit in each used currency is
defined by the [payment processor](#payment-processors) responsible for it.

### Structure

Promoter is a modular system which allows portal operators to run various payment processors as modules.

#### Payment processors

Payment processors are independent services which are wholly responsible for handling payments in a given
cryptocurrency (or any other way of gaining credits).

Since each payment processor is fully responsible for handling its currency, the processor is also responsible for
setting the price of a credit in their cryptocurrency (e.g. 1 credit == 1.025 SC) and converting from crypto to credits.

Each payment processor has two main responsibilities:

1. Issue new addresses to users.
2. Notify Promoter of any incoming payments on those addresses.

The reference payment processor implementation is [siacoin-promoter](https://github.com/SkynetLabs/siacoin-promoter)
which supports payment in Sia Coin (SC).

## API

The API is unfinished and should be extended for the needs of the upcoming UI.

### `GET /health`

This endpoint reports on the health of the service and its dependencies (the database).

Response:

```
{
  "dbAlive": <boolean>
}
```

Response status is always 200.

### `POST /payment`

This endpoint allows a payment processor to report an incoming payment.

Request:

```
{
  "txnID": <unique ID of the incoming transaction>, // string
  "sub": <user's sub>,                              // string
  "credits": <number of credits this user paid for> // float64
}
```

Response:

* status 204 on success
* status 400 or 500 on error

## UI

*To be implemented.*

### What the UI should provide

Promoter is supposed to expose a UI which lists the various price offerings of the portal. It should also allow the user
to change their current subscription. Currently, the backend does not expose API endpoints for these actions because
they haven't been sufficiently designed.

The UI should provide functionality for buying credits. This, generally, should consist of listing the various payment
processors supported by the portal and linking to their UIs. At the UI of the payment processor, the user should be able
to request a new address to which they can send cryptocurrency which will automatically be converted into credits for
them.

Optionally, the UI might list all transactions a user has made, potentially linking to a block explorer service.

### What the UI should NOT provide

Promoter's UI is not supposed to control the behaviour of any payment processors.
