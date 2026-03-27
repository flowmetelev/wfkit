const test = require("node:test");
const assert = require("node:assert/strict");

const {
    buildReleaseURL,
    repositorySlug,
    resolveReleaseAsset,
} = require("./install.js");

test("resolveReleaseAsset maps supported targets to release asset names", () => {
    assert.equal(resolveReleaseAsset("darwin", "arm64"), "wfkit-darwin-arm64");
    assert.equal(resolveReleaseAsset("darwin", "x64"), "wfkit-darwin-amd64");
    assert.equal(resolveReleaseAsset("linux", "arm64"), "wfkit-linux-arm64");
    assert.equal(resolveReleaseAsset("linux", "x64"), "wfkit-linux-amd64");
    assert.equal(resolveReleaseAsset("win32", "arm64"), "wfkit-windows-arm64.exe");
    assert.equal(resolveReleaseAsset("win32", "x64"), "wfkit-windows-amd64.exe");
});

test("resolveReleaseAsset rejects unsupported targets with a clear error", () => {
    assert.throws(
        () => resolveReleaseAsset("linux", "ia32"),
        /Unsupported platform linux\/ia32/
    );
});

test("buildReleaseURL uses the organization repository and versioned assets", () => {
    const url = buildReleaseURL("wfkit-linux-arm64", "1.2.3");

    assert.equal(
        url,
        `https://github.com/${repositorySlug}/releases/download/v1.2.3/wfkit-linux-arm64`
    );
});
