#!/usr/bin/env node

"use strict";

const fs = require("node:fs");
const path = require("node:path");

const npmDir = __dirname;
const repoRoot = path.resolve(npmDir, "..");
const mainPackage = require(path.join(npmDir, "package.json"));
const version = mainPackage.version;
const binaryBase = "agentchat";

const targets = [
  { nodeOS: "darwin", nodeArch: "arm64", goOS: "darwin", goArch: "arm64", binaryName: binaryBase },
  { nodeOS: "darwin", nodeArch: "x64", goOS: "darwin", goArch: "amd64", binaryName: binaryBase },
  { nodeOS: "linux", nodeArch: "arm64", goOS: "linux", goArch: "arm64", binaryName: binaryBase },
  { nodeOS: "linux", nodeArch: "x64", goOS: "linux", goArch: "amd64", binaryName: binaryBase },
  { nodeOS: "win32", nodeArch: "arm64", goOS: "windows", goArch: "arm64", binaryName: `${binaryBase}.exe` },
  { nodeOS: "win32", nodeArch: "x64", goOS: "windows", goArch: "amd64", binaryName: `${binaryBase}.exe` },
];

function argValue(name, fallback) {
  const idx = process.argv.indexOf(name);
  if (idx === -1) return fallback;
  const value = process.argv[idx + 1];
  if (!value || value.startsWith("--")) {
    throw new Error(`${name} requires a value`);
  }
  return path.resolve(value);
}

function packageOS(nodeOS) {
  return nodeOS === "win32" ? "windows" : nodeOS;
}

function packageName(target) {
  return `${mainPackage.name}-${packageOS(target.nodeOS)}-${target.nodeArch}`;
}

function packageDirName(target) {
  return packageName(target).replace("@renaissancemind/", "");
}

function sourceBinaryName(target) {
  const ext = target.goOS === "windows" ? ".exe" : "";
  return `${binaryBase}-v${version}-${target.goOS}-${target.goArch}${ext}`;
}

function writeJSON(file, data) {
  fs.writeFileSync(file, `${JSON.stringify(data, null, 2)}\n`);
}

function buildPackage(target, distDir, outDir) {
  const source = path.join(distDir, sourceBinaryName(target));
  if (!fs.existsSync(source)) {
    throw new Error(`missing release binary: ${source}`);
  }

  const dir = path.join(outDir, packageDirName(target));
  fs.rmSync(dir, { recursive: true, force: true });
  fs.mkdirSync(path.join(dir, "bin"), { recursive: true });

  const binaryPath = path.join(dir, "bin", target.binaryName);
  fs.copyFileSync(source, binaryPath);
  if (target.goOS !== "windows") {
    fs.chmodSync(binaryPath, 0o755);
  }

  writeJSON(path.join(dir, "package.json"), {
    name: packageName(target),
    version,
    description: `Platform binary for ${mainPackage.name} (${target.goOS}/${target.goArch}).`,
    homepage: mainPackage.homepage,
    repository: mainPackage.repository,
    license: mainPackage.license,
    author: mainPackage.author,
    os: [target.nodeOS],
    cpu: [target.nodeArch],
    files: ["bin", "README.md"],
    publishConfig: {
      access: "public",
    },
  });

  fs.writeFileSync(
    path.join(dir, "README.md"),
    `# ${packageName(target)}\n\n` +
      `Platform binary package for \`${mainPackage.name}\`.\n\n` +
      "Install the main package instead:\n\n" +
      "```bash\n" +
      "npm install -g @renaissancemind/agent-in-chat-feishu\n" +
      "```\n"
  );

  return dir;
}

function main() {
  const distDir = argValue("--dist", path.join(repoRoot, "dist"));
  const outDir = argValue("--out", path.join(distDir, "npm-platform"));

  fs.rmSync(outDir, { recursive: true, force: true });
  fs.mkdirSync(outDir, { recursive: true });

  for (const target of targets) {
    const dir = buildPackage(target, distDir, outDir);
    console.log(dir);
  }
}

if (require.main === module) {
  main();
}

module.exports = {
  packageName,
  sourceBinaryName,
  targets,
};
