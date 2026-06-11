# agent-in-chat-feishu

Run local AI coding agents inside Feishu/Lark chat loops.

## Install

```bash
npm install -g @renaissancemind/agent-in-chat-feishu@latest && agentchat setup feishu
```

The main package installs the matching platform binary from npm optional dependencies.
It does not download the CLI from GitHub Releases during install.

If the package is already installed, run:

```bash
agentchat setup feishu
```

## Quick Start

```bash
agentchat setup feishu
```

Without `--project`, setup uses Codex as the default agent, creates the local bot profile `feishu` with initial work directory `~/.agentchat/feishu/`, then installs and starts the background service. It also opens the Feishu permission page when possible and prints the direct permission confirmation link as the final step.

The user only needs to complete Feishu login or QR confirmation, approve the final permission link, add the bot to a chat, and mention it. Use `agentchat daemon status` or `agentchat daemon logs -f` to inspect the running service.

## Documentation

https://github.com/Renaissance-Mind/agent-in-chat-feishu
