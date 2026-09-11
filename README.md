# einvoicing

The command-line client for [einvoicing.dev](https://einvoicing.dev). Use it
to validate, convert and look up Peppol e-invoices without writing XML.

```sh
go install github.com/JustSteveKing/einvoicing-cli/cmd/einvoicing@latest
```

## Sign in

```sh
einvoicing login
```

- You enter your email address, and a six-digit code arrives by email.
- There is no password. Your first sign-in creates the account on the Free
  plan.
- Before the code is sent, the CLI solves a proof-of-work challenge. This
  takes a moment and keeps the sign-in endpoint from being abused.
- The API key is saved to your config directory, readable only by you.

In CI, set `EINVOICING_API_KEY` instead, and nothing is written to disk. A
test key (`einvoicing keys create ci --test`) is never metered, which makes it
the right key for CI.

## Use it

```sh
einvoicing validate invoice.xml                  # exit 1 if invalid: gate CI on it
einvoicing convert invoice.json -o invoice.xml   # JSON in, valid Peppol UBL out
einvoicing lookup 9932:GB123456789               # can this business receive Peppol invoices?
einvoicing usage                                 # usage against your plan
einvoicing upgrade --plan developer              # Stripe checkout
einvoicing billing                               # manage your subscription
einvoicing keys list | create <name> | revoke <id>
```

Add `--json` to any command to get the API's JSON instead of a summary.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | The document is invalid, or the invoice cannot be converted |
| 2 | The command itself failed: network, authentication, or bad input |

---

Peppol is a trademark of OpenPeppol AISBL. einvoicing.dev is independent and
not affiliated with or endorsed by OpenPeppol.
