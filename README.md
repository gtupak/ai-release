# AI Release

`airelease` creates GitHub releases from merged pull requests and can ask OpenRouter to suggest a semantic version and release title options.

## Features

- Interactive release creation flow
- Auto-discovers PRs since the latest release
- Uses `gh release create` to publish
- Global config in `~/.airelease/config.json`
- Per-repo configurable base branch
- Configurable OpenRouter model

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

## Commands

```bash
airelease
airelease config base <branch>
airelease config openrouter-api-key <api-key>
airelease config model <openrouter-model>
```

## Notes

- If OpenRouter is not configured, `airelease` still works without AI suggestions.
- API key is stored in `~/.airelease/config.json` with file mode `0600`.
