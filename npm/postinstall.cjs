#!/usr/bin/env node

"use strict";

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

console.log(lines.join("\n"));
