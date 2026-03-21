# HTTP API Reference

Base URL: `http://localhost:8080`

## Endpoints

### Create Todo
```
POST /api/todo
Content-Type: application/json

{
  "text": "fix the server",
  "tags": ["homelab", "deep-focus"],
  "source": "api",
  "urgent": false,
  "important": false,
  "note": "optional note text"
}

Response: 201 Created
{
  "id": "a1b2c3",
  "text": "fix the server",
  "tags": ["homelab", "deep-focus"],
  "source": "api",
  "status": "inbox",
  "created": "2026-03-21T14:32:00Z",
  "urgent": false,
  "important": false,
  "stale_count": 0
}
```

### List Todos
```
GET /api/todo?date=2026-03-21&tag=homelab&status=inbox&source=cli

Response: 200 OK
[{ ... todo objects ... }]
```

Query parameters (all optional):
- `date` — filter by creation date (YYYY-MM-DD)
- `tag` — filter by tag
- `status` — filter by status
- `source` — filter by source

### Edit Todo
```
PUT /api/todo/:id
Content-Type: application/json

{
  "text": "updated text",
  "tags": ["work"],
  "urgent": true,
  "important": false
}

Response: 200 OK
{ ... updated todo ... }
```

### Delete Todo
```
DELETE /api/todo/:id

Response: 204 No Content
```

### Change Status
```
PATCH /api/todo/:id/status
Content-Type: application/json

{
  "status": "today"
}

Response: 200 OK
{ ... updated todo ... }
```

### Bulk Create (Dump)
```
POST /api/dump
Content-Type: application/json

{
  "items": [
    {"text": "fix server", "tags": ["homelab"]},
    {"text": "buy groceries", "tags": ["errands"]}
  ],
  "source": "api",
  "default_tag": "braindump"
}

Response: 201 Created
{
  "created": 2,
  "todos": [{ ... }, { ... }]
}
```

### List Tags
```
GET /api/tags

Response: 200 OK
{
  "categories": ["homelab", "minecraft", ...],
  "energy": ["quick-win", "deep-focus", ...]
}
```

### Info Line
```
GET /api/info

Response: 200 OK
{
  "unprocessed": 12,
  "looping": 2
}
```

### Health Check
```
GET /api/health

Response: 200 OK
{"status": "ok"}
```

## Error Responses

```json
{
  "error": "todo not found",
  "code": 404
}
```

Common error codes:
- `400` — invalid request body or parameters
- `404` — todo or resource not found
- `422` — invalid tag (not in tags.yaml)
- `500` — internal server error
