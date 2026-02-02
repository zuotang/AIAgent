# Clear Data API Endpoint

## Overview

This endpoint allows you to clear all history records and memories for a specific user and agent combination.

## Endpoint

**POST** `/api/chat/clear`

## Request Body

```json
{
  "user_id": "string (required)",
  "agent_id": number (required, must be > 0)
}
```

### Parameters

- `user_id` (string, required): The ID of the user whose data should be cleared
- `agent_id` (number, required): The ID of the agent whose data should be cleared (must be greater than 0)

## Response

### Success Response (200 OK)

```json
{
  "success": true,
  "message": "Successfully cleared all data for user_id=<user_id> and agent_id=<agent_id>"
}
```

### Error Responses

#### 400 Bad Request - Invalid JSON

```json
{
  "error": "Invalid request",
  "message": "<error details>",
  "details": "Failed to parse JSON request body"
}
```

#### 400 Bad Request - Missing user_id

```json
{
  "error": "Validation failed",
  "message": "user_id field is required"
}
```

#### 400 Bad Request - Invalid agent_id

```json
{
  "error": "Validation failed",
  "message": "agent_id field is required and must be greater than 0"
}
```

#### 500 Internal Server Error

```json
{
  "error": "Failed to clear data",
  "message": "<error details>"
}
```

## What Gets Cleared

When you call this endpoint, the following data will be deleted:

1. **Chat History**: All chat messages between the user and agent
2. **Memories**: All structured memories (SQLite) stored for the user and agent
3. **Compressed Context**: Any compressed conversation context
4. **Vector Embeddings**: All semantic memories (Qdrant vectors) for the user and agent

## Example Usage

### Using cURL

```bash
curl -X POST http://localhost:8080/api/chat/clear \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user123",
    "agent_id": 1
  }'
```

### Using JavaScript (fetch)

```javascript
fetch('http://localhost:8080/api/chat/clear', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    user_id: 'user123',
    agent_id: 1
  })
})
.then(response => response.json())
.then(data => console.log(data))
.catch(error => console.error('Error:', error));
```

### Using Python (requests)

```python
import requests

url = 'http://localhost:8080/api/chat/clear'
data = {
    'user_id': 'user123',
    'agent_id': 1
}

response = requests.post(url, json=data)
print(response.json())
```

## Notes

- This operation is **irreversible**. All data will be permanently deleted.
- The endpoint uses database transactions to ensure data consistency.
- If vector deletion fails, the operation will still succeed for SQLite data, but a warning will be logged.
- Make sure the agent_id exists in your system before calling this endpoint.
