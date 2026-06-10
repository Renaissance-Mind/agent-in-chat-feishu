#!/usr/bin/env node

"use strict";

const { execFileSync } = require("child_process");
const path = require("path");
const fs = require("fs");

const PACKAGE = require("./package.json");
const NAME = "agentchat";

const TARGETS = {
  "darwin/arm64": {
    packageName: `${PACKAGE.name}-darwin-arm64`,
    binaryName: NAME,
  },
  "darwin/x64": {
    packageName: `${PACKAGE.name}-darwin-x64`,
    binaryName: NAME,
  },
  "linux/arm64": {
    packageName: `${PACKAGE.name}-linux-arm64`,
    binaryName: NAME,
  },
  "linux/x64": {
    packageName: `${PACKAGE.name}-linux-x64`,
    binaryName: NAME,
  },
  "win32/arm64": {
    packageName: `${PACKAGE.name}-windows-arm64`,
    binaryName: `${NAME}.exe`,
  },
  "win32/x64": {
    packageName: `${PACKAGE.name}-windows-x64`,
    binaryName: `${NAME}.exe`,
  },
};

function targetForPlatform(platform = process.platform, arch = process.arch) {
  const target = TARGETS[`${platform}/${arch}`];
  if (!target) {
    const supported = Object.keys(TARGETS).join(", ");
    throw new Error(
      `[agentchat] Unsupported platform: ${platform}/${arch}.\n` +
        `[agentchat] Supported platforms: ${supported}`
    );
  }
  return target;
}

function resolveBinaryPath(target = targetForPlatform(), baseDir = __dirname) {
  let packageJSONPath;
  try {
    packageJSONPath = require.resolve(`${target.packageName}/package.json`, { paths: [baseDir] });
  } catch (err) {
    throw new Error(
      `[agentchat] Missing platform package ${target.packageName}@${PACKAGE.version}.\n` +
        "[agentchat] Reinstall with optional dependencies enabled:\n" +
        "  npm install -g @renaissancemind/agent-in-chat-feishu"
    );
  }

  const binaryPath = path.join(path.dirname(packageJSONPath), "bin", target.binaryName);
  if (!fs.existsSync(binaryPath)) {
    throw new Error(
      `[agentchat] Platform package ${target.packageName}@${PACKAGE.version} did not contain ` +
        `${path.join("bin", target.binaryName)}.`
    );
  }
  return binaryPath;
}

function main(argv = process.argv.slice(2)) {
  const binaryPath = resolveBinaryPath();
  try {
    execFileSync(binaryPath, argv, { stdio: "inherit" });
  } catch (err) {
    if (typeof err.status === "number") {
      process.exit(err.status);
    }
    throw err;
  }
}

if (require.main === module) {
  main();
}

module.exports = {
  TARGETS,
  resolveBinaryPath,
  targetForPlatform,
};
