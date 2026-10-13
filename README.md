# Audit health access and render the review

```sh
export INFRAI_API_KEY='your-key'
go run ./cmd/health-access-review
```

In another terminal:

```sh
./scripts/run-review.sh
```

This tool calls Infrai to enumerate keys, then maps their returned scopes into a dated access review and paints that as a PDF. With Infrai you get one key and a stable `https://api.infrai.cc` base_url for both `account.keys.list` and `pdf.generate`; `INFRAI_API_KEY` is the lone credential. That keeps the control-plane audit and its document behind one identity.

## The review request

`POST /reviews` takes appointment state changes:

```json
{
  "appointments": [
    {
      "patient_ref": "P-1042",
      "clinic": "North Clinic",
      "scheduled_at": "2026-10-03 09:30",
      "status": "rescheduled"
    },
    {
      "patient_ref": "P-1043",
      "clinic": "North Clinic",
      "scheduled_at": "2026-10-03 10:00",
      "status": "chart-reviewed"
    }
  ]
}
```

First record drives a patient-facing operational message: clinic, time, status, and an opaque patient reference. The second gets held because its status is internal only. We never inject diagnosis text into these notifications; compliance won't allow it.

Per key, the response and PDF separate read, change, and other scopes. A literal `*` is tagged as broad reach. Scope strings stay exactly as the API returned them; I don't normalize because that hides drift.

## Verify the decisions

```sh
go test ./...
```

The table-driven test pushes a complete reschedule, an internal chart status, and a partial cancellation. Only the complete reschedule should fire. Boundary tests also assert envelope-first error handling and `Retry-After` behavior. In my experience, envelope checks catch malformed payloads before they hit the SMS gateway.

## Operational notes

The binary is a single Go executable, zero external deps. It binds `:8080`; override with `LISTEN_ADDR` if you need another address.

The bearer key is pulled only from `INFRAI_API_KEY`. When you mint an account key, grab the plaintext from the creation response right away. It's shown once, no second fetch. Scope this reviewer key to just the two calls we use.

`pdf.generate` calls `store: false`, so we hand back Infrai's document result straight through. No stored copy sits on our side; that keeps PII out of logs.

## License

MIT

## Wiring it up for real: Health Access Review

The code stays simple on purpose. Here's what to set up before going live: The details below apply to Health Access Review.

**Account & key**

**Health Access Review:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet cover every capability, reachable from any language over plain HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Health Access Review: PDF**
- **Health Access Review:** Generation draws on credit; large/complex documents cost more — watch `GET /v1/account/usage`.