const { join } = require("path");
const { createWriteStream, existsSync, mkdirSync } = require("fs");
const { promisify } = require("util");
const { exec } = require("child_process");
const https = require("https");
const packageJSON = require("./package.json");

const pipeline = promisify(require("stream").pipeline);
const repositorySlug = "flowmetelev/wfkit";
const supportedTargets = new Map([
  ["darwin:arm64", "wfkit-darwin-arm64"],
  ["darwin:x64", "wfkit-darwin-amd64"],
  ["linux:arm64", "wfkit-linux-arm64"],
  ["linux:x64", "wfkit-linux-amd64"],
  ["win32:arm64", "wfkit-windows-arm64.exe"],
  ["win32:x64", "wfkit-windows-amd64.exe"],
]);

function resolveReleaseAsset(platform = process.platform, arch = process.arch) {
  const targetKey = `${platform}:${arch}`;
  const binaryName = supportedTargets.get(targetKey);

  if (!binaryName) {
    const supportedList = Array.from(supportedTargets.keys())
      .map((target) => target.replace(":", "/"))
      .join(", ");
    throw new Error(
      `Unsupported platform ${platform}/${arch}. Supported targets: ${supportedList}.`,
    );
  }

  return binaryName;
}

function resolveVersion() {
  return process.env.npm_package_version || packageJSON.version;
}

function buildReleaseURL(binaryName, version = resolveVersion()) {
  const normalizedVersion = String(version || "").trim();
  if (!normalizedVersion) {
    throw new Error("Package version is empty, cannot resolve release URL.");
  }

  return `https://github.com/${repositorySlug}/releases/download/v${normalizedVersion}/${binaryName}`;
}

function download(url, destPath) {
  return new Promise((resolve, reject) => {
    const request = (downloadUrl, redirectCount = 0) => {
      if (redirectCount > 5) {
        reject(new Error("Too many redirects while downloading the binary."));
        return;
      }

      https
        .get(
          downloadUrl,
          {
            headers: {
              "user-agent": "wfkit-npm-installer",
            },
          },
          (response) => {
            if (
              response.statusCode >= 300 &&
              response.statusCode < 400 &&
              response.headers.location
            ) {
              response.resume();
              request(response.headers.location, redirectCount + 1);
              return;
            }

            if (response.statusCode !== 200) {
              response.resume();
              reject(
                new Error(
                  `Failed to download binary: HTTP ${response.statusCode} ${response.statusMessage || ""}`.trim(),
                ),
              );
              return;
            }

            const fileStream = createWriteStream(destPath);
            pipeline(response, fileStream).then(resolve).catch(reject);
          },
        )
        .on("error", reject);
    };

    request(url);
  });
}

async function downloadBinary() {
  const platform = process.platform;
  const arch = process.arch;
  const binaryName = resolveReleaseAsset(platform, arch);
  const version = resolveVersion();
  const url = buildReleaseURL(binaryName, version);
  const binDir = join(__dirname, "bin");

  if (!existsSync(binDir)) {
    mkdirSync(binDir, { recursive: true });
  }

  const destPath = join(binDir, platform === "win32" ? "wfkit.exe" : "wfkit");

  console.log(`Downloading wfkit binary for ${platform} (${arch})...`);
  console.log(`From: ${url}`);

  await download(url, destPath);

  if (platform !== "win32") {
    await promisify(exec)(`chmod +x "${destPath}"`);
  }

  console.log("wfkit installed successfully!");
}

module.exports = {
  buildReleaseURL,
  downloadBinary,
  repositorySlug,
  resolveReleaseAsset,
  resolveVersion,
  supportedTargets,
};

if (require.main === module) {
  downloadBinary().catch((err) => {
    console.error("Failed to install wfkit:", err.message);
    process.exit(1);
  });
}
