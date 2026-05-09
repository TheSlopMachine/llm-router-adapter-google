# Google AI Studio Adapter for llm-router

Official adapter for Google AI Studio (Gemini API) integration with llm-router.

## Installation

Add to your `adapters.conf`:

```
github.com/TheSlopMachine/llm-router-adapter-google latest
```

## Configuration

1. Get an API key from [Google AI Studio](https://aistudio.google.com/app/apikey)
2. Add credential in llm-router dashboard:
   ```json
   {
     "api_key": "your-api-key-here"
   }
   ```

## Supported Models

- `google/gemini-2.5-flash` - Fast, cost-effective
- `google/gemini-3-flash-preview` - Latest flash model
- `google/gemini-3.1-pro-preview` - Most capable model
- `google/gemini-3.1-flash-lite` - High-volume workloads

## Features

- Non-streaming completions
- Streaming completions (SSE)
- Multi-turn conversations
- Automatic error handling and retry
- Rate limit detection
- Credential rotation support

## Example Usage

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "google/gemini-2.5-flash",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

## Rate Limits

| Model | RPM | TPM | RPD |
|-------|-----|-----|-----|
| gemini-2.5-flash | 1000 | 1M | 1500 |
| gemini-3-flash-preview | 1000 | 1M | 1500 |
| gemini-3.1-pro-preview | 500 | 500K | 1000 |
| gemini-3.1-flash-lite | 2000 | 2M | 3000 |

## License

MIT
