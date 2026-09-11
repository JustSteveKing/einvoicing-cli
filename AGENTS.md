# AGENTS.md

Guidance for coding agents working in this repository. `README.md` is for
people using the CLI; this file is for people changing it.

## Commands

```bash
make check      # gofmt + go.mod tidiness + vet + race tests; what CI runs
go run ./cmd/einvoicing --api-url http://localhost validate some.xml
```

Go 1.27. The only dependency is cobra, and that is deliberate.

## Architecture

```
internal/altcha        ALTCHA v2 solver (PBKDF2), for sign-in
internal/api           client for openapi.yaml; errors are *api.Problem (RFC 9457)
internal/credentials   the saved key: 0600 file, atomic writes, EINVOICING_API_KEY overrides
cmd/einvoicing         cobra commands
```

The API contract is `openapi.yaml` in the shared `einvoicing.dev` repo. The
client covers exactly its operations, and the types in `internal/api`
mirror its schemas.

## Decisions worth not relitigating

**The challenge is echoed back verbatim.** The server verifies its HMAC over
the parameters it parses from our payload. Re-serialising them (reordering,
or dropping a field the solver does not use, such as `keySignature`) breaks a
correct solution. So `altcha.Solve` takes and returns the raw JSON.

**The solver is checked against the PHP library, not itself.** The vector in
`altcha_test.go` was created, solved and verified by altcha-org/altcha 2.1,
the library the API uses.

**Amounts are strings.** The API sends decimals as strings, and the CLI never
does arithmetic on money.

**Exit 1 means "the answer is no", exit 2 means "it broke".** CI needs to
tell an invalid invoice from a broken pipeline.
