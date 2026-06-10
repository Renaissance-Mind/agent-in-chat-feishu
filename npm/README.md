# agent-in-chat-feishu

Run local AI coding agents inside Feishu/Lark chat loops.

## Install

```bash
npm install -g @renaissancemind/agent-in-chat-feishu
```

The main package installs the matching platform binary from npm optional dependencies.
It does not download the CLI from GitHub Releases during install.

The installed CLI command is:

```bash
agentchat
```

## Quick Start

```bash
agentchat setup feishu
```

Without `--project`, setup uses Codex as the default agent, creates the local bot profile `feishu` with initial work directory `~/.agentchat/feishu/`, then installs and starts the background service. It also opens the Feishu permission page when possible and prints the direct permission confirmation link as the final step. Use `agentchat daemon status` or `agentchat daemon logs -f` to inspect it.

## Documentation

https://github.com/Renaissance-Mind/agent-in-chat-feishu
