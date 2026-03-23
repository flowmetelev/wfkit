const { join } = require("path");
const { createWriteStream, existsSync, mkdirSync } = require("fs");
const { promisify } = require("util");
const { exec } = require("child_process");
const https = require("https");

const pipeline = promisify(require("stream").pipeline);

async function downloadBinary() {
    const platform = process.platform;
    const arch = process.arch;

    let binaryName;

    // Map platform and architecture to release binary names
    if (platform === "darwin") {
        binaryName = arch === "arm64" ? "wfkit-darwin-arm64" : "wfkit-darwin-amd64";
    } else if (platform === "linux") {
        // Currently only amd64 is compiled for linux
        binaryName = "wfkit-linux-amd64";
    } else if (platform === "win32") {
        // Currently only amd64 is compiled for windows
        binaryName = "wfkit-windows-amd64.exe";
    } else {
        throw new Error(`Unsupported platform: ${platform}`);
    }

    const version = process.env.npm_package_version || "latest";
    const tag = version === "latest" ? "latest" : `v${version}`;
    const url = `https://github.com/yndmitry/wfkit/releases/${tag === "latest" ? "latest/download" : `download/${tag}`}/${binaryName}`;

    const binDir = join(__dirname, "bin");
    if (!existsSync(binDir)) {
        mkdirSync(binDir, { recursive: true });
    }

    const destPath = join(
        binDir,
        platform === "win32" ? "wfkit.exe" : "wfkit"
    );

    console.log(`Downloading wfkit binary for ${platform} (${arch})...`);
    console.log(`From: ${url}`);

    await new Promise((resolve, reject) => {
        // Handle redirects manually since native https doesn't follow them automatically
        const request = (downloadUrl) => {
            https.get(downloadUrl, (response) => {
                if (response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
                    request(response.headers.location);
                } else if (response.statusCode === 200) {
                    const fileStream = createWriteStream(destPath);
                    pipeline(response, fileStream)
                        .then(resolve)
                        .catch(reject);
                } else {
                    reject(new Error(`Failed to download binary: HTTP ${response.statusCode} - ${response.statusMessage}`));
                }
            }).on('error', reject);
        };
        request(url);
    });

    if (platform !== "win32") {
        await promisify(exec)(`chmod +x "${destPath}"`);
    }

    console.log("wfkit installed successfully!");
}

downloadBinary().catch((err) => {
    console.error("Failed to install wfkit:", err.message);
    process.exit(1);
});
