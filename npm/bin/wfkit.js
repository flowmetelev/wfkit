#!/usr/bin/env node

const { existsSync } = require("fs");
const { join } = require("path");
const { spawnSync } = require("child_process");

const platform = process.platform;
const binaryName = platform === "win32" ? "wfkit.exe" : "wfkit";
const binaryPath = join(__dirname, binaryName);

if (!existsSync(binaryPath)) {
  console.error(
    `wfkit binary is missing at ${binaryPath}. Reinstall the package to run the postinstall download again.`,
  );
  process.exit(1);
}

const result = spawnSync(binaryPath, process.argv.slice(2), {
  stdio: "inherit",
});

if (result.error) {
  console.error(`Failed to start wfkit: ${result.error.message}`);
  process.exit(1);
}

process.exit(result.status === null ? 1 : result.status);
