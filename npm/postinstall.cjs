#!/usr/bin/env node

"use strict";

const fs = require("node:fs");

const lines = [
  "",
  "+------------------------------------------------------------+",
  "| agentchat installed                                        |",
  "+------------------------------------------------------------+",
  "",
  "Next step:",
  "  agentchat setup feishu",
  "",
  "Defaults:",
  "  Agent:   Codex",
  "  Profile: feishu",
  "  Workdir: ~/.agentchat/feishu",
  "  Service: setup installs and starts the daemon automatically",
  "",
  "After setup, agentchat will open the Feishu permission page and",
  "print the direct permission confirmation link as the final step.",
  "",
];

const message = lines.join("\n");

function writeDirectlyToTerminal(text) {
  const terminalPath = process.platform === "win32" ? "\\\\.\\CONOUT$" : "/dev/tty";

  try {
    const fd = fs.openSync(terminalPath, "w");
    try {
      fs.writeSync(fd, text);
    } finally {
      fs.closeSync(fd);
    }
    return true;
  } catch {
    return false;
  }
}

if (
  process.env.AGENTCHAT_POSTINSTALL_FORCE_STDOUT !== "1" &&
  !process.stdout.isTTY &&
  writeDirectlyToTerminal(message)
) {
  process.exit(0);
}

process.stdout.write(message);
