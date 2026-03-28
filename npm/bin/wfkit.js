#!/usr/bin/env node

const { existsSync } = require("fs");
const { join } = require("path");
const { spawnSync } = require("child_process");
const { downloadBinary } = require("../install.js");

const platform = process.platform;
const binaryName = platform === "win32" ? "wfkit.exe" : "wfkit";
const binaryPath = join(__dirname, binaryName);

function resolveBinaryPath() {
  const overridePath = process.env.WFKIT_BINARY_PATH;
  if (!overridePath) {
    return binaryPath;
  }

  return overridePath;
}

async function main() {
  const resolvedBinaryPath = resolveBinaryPath();

  if (process.env.WFKIT_BINARY_PATH && !existsSync(resolvedBinaryPath)) {
    console.error(
      `WFKIT_BINARY_PATH points to a missing binary: ${resolvedBinaryPath}`,
    );
    process.exit(1);
  }

  if (!process.env.WFKIT_BINARY_PATH && !existsSync(resolvedBinaryPath)) {
    console.warn("wfkit binary is missing. Attempting to download it now.");

    try {
      await downloadBinary();
    } catch (error) {
      console.error(
        `Failed to download the wfkit binary on first run: ${error.message}`,
      );
      process.exit(1);
    }
  }

  const result = spawnSync(resolvedBinaryPath, process.argv.slice(2), {
    stdio: "inherit",
  });

  if (result.error) {
    console.error(`Failed to start wfkit: ${result.error.message}`);
    process.exit(1);
  }

  process.exit(result.status === null ? 1 : result.status);
}

module.exports = {
  binaryPath,
  resolveBinaryPath,
};

if (require.main === module) {
  main();
}
