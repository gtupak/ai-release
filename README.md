# AI Release

`airelease` creates GitHub releases from merged pull requests and can ask OpenRouter (or custom endpoints) to suggest a semantic version and release title options.

## Features

- Interactive release creation flow
- Auto-discovers PRs since the latest release
- Uses `gh release create` to publish
- Global config in `~/.airelease/config.json`
- Per-repo configurable base branch
- Configurable OpenRouter model or custom endpoint
- Dry run mode to preview changes

## Requirements

- `git`
- `gh` (authenticated with `gh auth login`)
- Go 1.22+

## Install

From this repository:

```bash
go install .
```

Ensure your Go bin path is in `PATH`:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

## Quick Start

### Using OpenRouter

1) Set your OpenRouter API key:

```bash
airelease config openrouter-api-key <your-api-key>
```

2) Optional: set model globally:

```bash
airelease config model qwen/qwen3.5-flash-02-23
```

3) Optional: set base branch for current repo:

```bash
airelease config base develop
```

4) Run it inside a git repository:

```bash
airelease
```

### Using a Custom Endpoint

1) Set your custom endpoint:

```bash
airelease config model-url "https://your-endpoint.com/v1"
```

2) Add required headers (e.g., Authorization):

```bash
airelease config model-header "Authorization: Bearer your-token"
```

3) Set your model name:

```bash
airelease config model "your-model-name"
```

4) Run it:

```bash
airelease
```

**Dry Run Mode**

Test without creating a release:

```bash
airelease --dry-run
```

## Commands

### Basic Usage

```bash
airelease                    # Create a GitHub release from merged PRs
airelease --dry-run          # Preview release without creating it
```

### Configuration

```bash
airelease config base <branch>                    # Default base branch for current repo
airelease config openrouter-api-key <api-key>     # Set OpenRouter API key
airelease config openrouter <true|false>          # Enable/disable OpenRouter mode  
airelease config model <model-name>               # Set model name
```

### Custom LLM Endpoints

```bash
airelease config model-url <url>                 # Set custom endpoint (full path)
airelease config model-header <key:value>        # Add custom header
airelease config model-header remove <key>       # Remove a header
airelease config model-header list               # List all custom headers
```

**Note:** When using `model-url` or `model-header`, OpenRouter mode is automatically disabled. The tool will append `/chat/completions` to the base URL if not present.

## Notes

- If OpenRouter is not configured, `airelease` still works without AI suggestions.
- API key is stored in `~/.airelease/config.json` with file mode `0600`.
- Custom endpoints are supported with the `model-url` and `model-header` config commands.
- `dry-run` mode previews the release without actually creating it.
