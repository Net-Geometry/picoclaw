# Manus Channel Integration

## Overview

The Manus channel enables PicoClaw to send messages to [Manus AI](https://manus.im/), a comprehensive AI agent platform that can plan, execute multi-step workflows, use tools, browse the web, and deliver results in multiple formats.

**Manus API Documentation:** https://open.manus.im/docs

## Features

- **Task Creation**: Submit complex queries and tasks to Manus
- **Result Polling**: Automatically polls for task completion with configurable intervals
- **Error Handling**: Graceful error handling with retry logic
- **Async Task Processing**: Tasks run asynchronously with timeout protection

## Configuration

### Basic Setup

1. Get your Manus API key from [Manus API Integration Settings](https://manus.im/app?show_settings=integrations&app_name=api)
2. Add it to your PicoClaw config:

```json
{
  "channel_list": {
    "manus": {
      "enabled": true,
      "type": "manus",
      "settings": {
        "api_key": "YOUR_MANUS_API_KEY",
        "api_base": "https://api.manus.ai",
        "poll_interval": 2000,
        "task_timeout": 300
      }
    }
  }
}
```

### Configuration Options

| Option          | Type         | Default                | Description                                        |
| --------------- | ------------ | ---------------------- | -------------------------------------------------- |
| `api_key`       | SecureString | *required*             | Your Manus API key (must be set)                   |
| `api_base`      | String       | `https://api.manus.ai` | Manus API base URL                                 |
| `poll_interval` | Integer      | 2000                   | How often to poll for results (milliseconds)       |
| `task_timeout`  | Integer      | 300                    | Maximum time to wait for task completion (seconds) |

### Environment Variables

You can also configure Manus via environment variables:

```bash
export PICOCLAW_CHANNELS_MANUS_API_KEY="your-api-key"
export PICOCLAW_CHANNELS_MANUS_API_BASE="https://api.manus.ai"
export PICOCLAW_CHANNELS_MANUS_POLL_INTERVAL="2000"
export PICOCLAW_CHANNELS_MANUS_TASK_TIMEOUT="300"
```

## Usage

### Sending a Message to Manus

```bash
# CLI mode
picoclaw agent -c manus -m "Analyze the latest AI trends and create a summary"
```

### How It Works

1. **Task Creation**: Your message is submitted as a new task to Manus API
2. **Processing**: Manus processes the task asynchronously, potentially using:
   - Web browsing
   - Tool execution
   - Multi-step workflows
   - AI reasoning and analysis
3. **Result Polling**: PicoClaw polls for results every `poll_interval` milliseconds
4. **Timeout**: If no result within `task_timeout` seconds, the operation completes with a timeout error
5. **Response**: The result (or error) is logged and the task ID is returned

### Example Flows

#### Research Query
```
User: "Manus, what are the latest developments in quantum computing?"
→ Task created at Manus
→ Manus browses web, gathers information
→ Results returned and displayed
```

#### Content Creation
```
User: "Create a marketing plan for a new SaaS product"
→ Task created at Manus
→ Manus plans approach, gathers market data
→ Creates comprehensive plan document
```

## Architecture

### Message Flow

```
PicoClaw Send()
    ↓
CreateTask() [POST /v2/task.create]
    ↓
Get Task ID
    ↓
PollTaskResult()
    ├─ Get /v2/task.listMessages
    ├─ Parse events (status_update, assistant_message, error_message)
    ├─ Check status (running, stopped, waiting, error)
    └─ Return when stopped or timeout
    ↓
Log Results & Return
```

### Task Lifecycle

- **running**: Task is being processed - continue polling
- **stopped**: Task completed - extract and return results
- **waiting**: Task awaiting confirmation or input (logged but not interactively handled)
- **error**: Task failed - error message returned

## Limitations

### Current Version

- **Output-only**: Messages are sent to Manus, but Manus cannot initiate messages to PicoClaw
- **Synchronous polling**: Uses polling instead of webhooks (future enhancement)
- **No interactive confirmations**: Tasks requiring user confirmation are logged but not interactively handled
- **Single API key**: Only one Manus account per channel instance

### Future Enhancements

- [ ] Webhook support for real-time task updates
- [ ] Interactive confirmation handling (deployments, emails, calendar events, etc.)
- [ ] Browser connection management
- [ ] Multiple concurrent tasks
- [ ] Result caching

## API Reference

### Task Creation

```
POST /v2/task.create
Header: x-manus-api-key: <YOUR_API_KEY>
Content-Type: application/json

Body: {
  "message": {
    "content": "Your task description"
  }
}

Response: {
  "ok": true,
  "task_id": "task_abc123",
  "request_id": "req_def456"
}
```

### Poll Task Messages

```
GET /v2/task.listMessages?task_id=<TASK_ID>&order=desc&limit=100
Header: x-manus-api-key: <YOUR_API_KEY>

Response: {
  "ok": true,
  "messages": [
    {
      "type": "status_update",
      "status_update": {
        "agent_status": "stopped",
        "status_detail": {...}
      }
    },
    {
      "type": "assistant_message",
      "assistant_message": {
        "content": "Result content..."
      }
    }
  ]
}
```

## Troubleshooting

### Task Creation Fails

**Error**: `create task failed: status 401`

- Check if your API key is valid
- Verify you're using the correct API base URL
- Check the Manus API documentation for authentication requirements

### No Results Returned

**Check these**:

1. Is `task_timeout` long enough for your task?
2. Are you polling within `poll_interval`?
3. Check logs for task status messages (especially "waiting" events)

### High Latency

**Recommendations**:

- Increase `poll_interval` if you don't need real-time updates (reduces API calls)
- Increase `task_timeout` if tasks are taking longer than expected
- Consider using webhook integration (future feature)

## Logging

Enable debug logging to see detailed channel behavior:

```json
{
  "logger": {
    "level": "debug"
  }
}
```

Debug logs will show:
- Task creation details
- Poll status and responses
- Result extraction
- Error conditions

## Security

### API Key Management

- **Store securely**: Use environment variables or secure vaults, never commit to git
- **Regenerate regularly**: Rotate API keys periodically
- **Limit scope**: If Manus supports scoped keys, use minimal permissions
- **Monitor usage**: Watch for unexpected API usage patterns

### Data Privacy

- All API calls are made over HTTPS
- Task content is transmitted to Manus - ensure compliance with data privacy requirements
- Results are not cached persistently by PicoClaw

## Examples

### Research Analysis

```bash
# Send complex research request
picoclaw agent -c manus -m \
  "Find the top 5 AI model architectures published in the last 6 months, \
   analyze their innovations, and create a comparison table"
```

### Data Processing

```bash
# Process data with Manus
picoclaw agent -c manus -m \
  "Process this CSV data: [data], create visualizations and statistical analysis"
```

### Documentation Generation

```bash
# Generate documentation
picoclaw agent -c manus -m \
  "Create API documentation for a REST service with these endpoints: [list]"
```

## References

- [Manus API Docs](https://open.manus.im/docs)
- [Manus Website](https://manus.im/)
- [PicoClaw Channel Guide](../README.md)
