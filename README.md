# Fees API

A billing API built with Encore and Temporal.

Each bill is represented as a long running Temporal workflow. It stays open while fees accrue, and closes when the billing period ends which point the final invoice is available.

## Running locally

```bash
# Terminal 1 — Temporal dev server
temporal server start-dev

# Terminal 2 — Encore app
encore run
```

## API

| Method |       Path         |          Description                        | 
|--------|--------------------|---------------------------------------------|
| `POST` | `/bills`           | Create a new bill (starts the workflow)     |
| `POST` | `/bills/:id/items` | Add a line item to an open bill             |
| `POST` | `/bills/:id/close` | Close the bill and return the final invoice |
| `GET`  | `/bills/:id`       | Get the current state of a bill             |

### Create a bill

```bash
curl -X POST http://localhost:4000/bills \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"cust-123","period_start":"2026-08"}'
```

### Add a line item

```bash
curl -X POST http://localhost:4000/bills/{bill_id}/items \
  -H "Content-Type: application/json" \
  -d '{"client_item_id":"inv-001","description":"Monthly fee","amount":2999,"currency":"USD"}'
```

`client_item_id` is optional. If you send the same one twice, because a request timed out and got retried. the fee only gets counted once.

### Close a bill

```bash
curl -X POST http://localhost:4000/bills/{bill_id}/close
```

Returns the final invoice: every line item, plus a total per currency.

## Why it's built this way

**Temporal** 
Because a bill isn't one request and response. It's something that stays open for days or even weeks while fees come in slowly. Using a workflow means:
- If the service restarts nothing gets lost. Temporal picks up the bill where it left off.
- Fees never overlap. Temporal handles one signal at a time for each bill so there's no chance of two calls racing each other.
- Once the bill is closed the workflow stops accepting signals. Thats how we enforce "no charges after close”. Not by relying to remember to add checks everywhere.

**Signals**
for adding line items and closing the bill. Signals get written to Temporal history the moment they arrive, before the workflow even processes them so nothing gets dropped, even if the worker crashes mid request.

**Money as integers**
`$29.99` is stored as `2999` (cents). Floats can't represent money exactly, and small rounding errors add up fast when summing many line items.

**Multi currency**
by tracking each currency total separately (`{"USD": 2999, "GEL": 1500}`) rather than trying to convert between them. I left conversion for a downstream system that actually needs it.

**Idempotency key on line items** 
`client_item_id`, because retries are inevitable in a distributed system, and a billing API is exactly the place where double charging is unacceptable.
