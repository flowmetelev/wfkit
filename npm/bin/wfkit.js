#!/usr/bin/env node

const { existsSync } = require("fs");
const { join } = require("path");
const { spawnSync } = require("child_process");
const { downloadBinary } = require("../install.js");

const platform = process.platform;
const binaryName = platform === "win32" ? "wfkit.exe" : "wfkit";
const binaryPath = join(__dirname, binaryName);

async function main() {
  if (!existsSync(binaryPath)) {
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

  const result = spawnSync(binaryPath, process.argv.slice(2), {
    stdio: "inherit",
  });

  if (result.error) {
    console.error(`Failed to start wfkit: ${result.error.message}`);
    process.exit(1);
  }

  process.exit(result.status === null ? 1 : result.status);
}

main();
