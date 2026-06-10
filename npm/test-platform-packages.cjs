#!/usr/bin/env node

"use strict";

const assert = require("node:assert/strict");
const { execFileSync } = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

const npmDir = __dirname;
const pkg = require("./package.json");

const targets = [
  ["darwin", "arm64", "darwin", "arm64", "agentchat"],
  ["darwin", "x64", "darwin", "amd64", "agentchat"],
  ["linux", "arm64", "linux", "arm64", "agentchat"],
  ["linux", "x64", "linux", "amd64", "agentchat"],
  ["win32", "arm64", "windows", "arm64", "agentchat.exe"],
  ["win32", "x64", "windows", "amd64", "agentchat.exe"],
];

function platformPackageName(osName, archName) {
  return `${pkg.name}-${osName === "win32" ? "windows" : osName}-${archName}`;
}

test("main npm package depends on platform-specific binary packages", () => {
  assert.equal(pkg.scripts && pkg.scripts.postinstall, "node postinstall.cjs");
  assert.deepEqual(pkg.files, ["run.js", "postinstall.cjs", "README.md"]);

  const expectedDeps = {};
  for (const [nodeOS, nodeArch] of targets) {
    expectedDeps[platformPackageName(nodeOS, nodeArch)] = pkg.version;
  }
  assert.deepEqual(pkg.optionalDependencies, expectedDeps);
});

test("postinstall message points users to setup feishu", () => {
  const output = execFileSync(process.execPath, [path.join(npmDir, "postinstall.cjs")], {
    cwd: npmDir,
    encoding: "utf8",
  });
  assert.match(output, /agentchat setup feishu/);
  assert.match(output, /Codex/);
});

test("build-platform-packages creates npm packages from release binaries", () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "agentchat-platform-packages-"));
  const distDir = path.join(tmp, "dist");
  const outDir = path.join(tmp, "out");
  fs.mkdirSync(distDir);

  for (const [, , goOS, goArch, binaryName] of targets) {
    const sourceName = `agentchat-v${pkg.version}-${goOS}-${goArch}${binaryName.endsWith(".exe") ? ".exe" : ""}`;
    fs.writeFileSync(path.join(distDir, sourceName), `${goOS}/${goArch}\n`);
  }

  execFileSync(
    process.execPath,
    [path.join(npmDir, "build-platform-packages.cjs"), "--dist", distDir, "--out", outDir],
    { cwd: npmDir, stdio: "pipe" }
  );

  for (const [nodeOS, nodeArch, goOS, goArch, binaryName] of targets) {
    const packageName = platformPackageName(nodeOS, nodeArch);
    const packageDir = path.join(outDir, packageName.replace("@renaissancemind/", ""));
    const packageJSON = JSON.parse(fs.readFileSync(path.join(packageDir, "package.json"), "utf8"));

    assert.equal(packageJSON.name, packageName);
    assert.equal(packageJSON.version, pkg.version);
    assert.deepEqual(packageJSON.os, [nodeOS]);
    assert.deepEqual(packageJSON.cpu, [nodeArch]);
    assert.deepEqual(packageJSON.files, ["bin", "README.md"]);

    const binaryPath = path.join(packageDir, "bin", binaryName);
    assert.equal(fs.readFileSync(binaryPath, "utf8"), `${goOS}/${goArch}\n`);
  }
});

test("run.js resolves the current platform package binary", () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "agentchat-run-resolve-"));
  const packageDir = path.join(
    tmp,
    "node_modules",
    "@renaissancemind",
    "agent-in-chat-feishu-darwin-arm64"
  );
  fs.mkdirSync(path.join(packageDir, "bin"), { recursive: true });
  fs.writeFileSync(
    path.join(packageDir, "package.json"),
    JSON.stringify({ name: "@renaissancemind/agent-in-chat-feishu-darwin-arm64", version: pkg.version })
  );
  fs.writeFileSync(path.join(packageDir, "bin", "agentchat"), "binary\n");

  const runner = require("./run.js");
  const target = runner.targetForPlatform("darwin", "arm64");
  assert.equal(target.packageName, "@renaissancemind/agent-in-chat-feishu-darwin-arm64");
  assert.equal(
    fs.realpathSync(runner.resolveBinaryPath(target, tmp)),
    fs.realpathSync(path.join(packageDir, "bin", "agentchat"))
  );
});

test("run.js fails clearly when a platform package has no binary", () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "agentchat-run-missing-binary-"));
  const packageDir = path.join(
    tmp,
    "node_modules",
    "@renaissancemind",
    "agent-in-chat-feishu-darwin-arm64"
  );
  fs.mkdirSync(packageDir, { recursive: true });
  fs.writeFileSync(
    path.join(packageDir, "package.json"),
    JSON.stringify({ name: "@renaissancemind/agent-in-chat-feishu-darwin-arm64", version: pkg.version })
  );

  const runner = require("./run.js");
  const target = runner.targetForPlatform("darwin", "arm64");
  assert.throws(
    () => runner.resolveBinaryPath(target, tmp),
    /Platform package .* did not contain/
  );
});
