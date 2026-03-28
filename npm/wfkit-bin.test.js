const test = require("node:test");
const assert = require("node:assert/strict");

const { binaryPath, resolveBinaryPath } = require("./bin/wfkit.js");

test("resolveBinaryPath uses the packaged binary by default", () => {
  const previousValue = process.env.WFKIT_BINARY_PATH;
  delete process.env.WFKIT_BINARY_PATH;

  try {
    assert.equal(resolveBinaryPath(), binaryPath);
  } finally {
    if (previousValue !== undefined) {
      process.env.WFKIT_BINARY_PATH = previousValue;
    }
  }
});

test("resolveBinaryPath respects WFKIT_BINARY_PATH override", () => {
  const previousValue = process.env.WFKIT_BINARY_PATH;
  process.env.WFKIT_BINARY_PATH = "/tmp/custom-wfkit";

  try {
    assert.equal(resolveBinaryPath(), "/tmp/custom-wfkit");
  } finally {
    if (previousValue === undefined) {
      delete process.env.WFKIT_BINARY_PATH;
    } else {
      process.env.WFKIT_BINARY_PATH = previousValue;
    }
  }
});
