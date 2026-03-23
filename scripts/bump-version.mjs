#!/usr/bin/env node

import fs from "node:fs"
import path from "node:path"

const packagePath = path.resolve("npm/package.json")
const raw = fs.readFileSync(packagePath, "utf8")
const pkg = JSON.parse(raw)

const input = process.argv[2]
const dryRun = process.argv.includes("--dry-run")

if (!input) {
  console.error("Usage: node scripts/bump-version.mjs <patch|minor|major|x.y.z> [--dry-run]")
  process.exit(1)
}

const current = pkg.version
const next = bumpVersion(current, input)

if (current === next) {
  console.error(`Version is already ${current}`)
  process.exit(1)
}

if (dryRun) {
  console.log(`${current} -> ${next}`)
  process.exit(0)
}

pkg.version = next
fs.writeFileSync(packagePath, JSON.stringify(pkg, null, 4) + "\n")

console.log(`${current} -> ${next}`)

function bumpVersion(version, mode) {
  if (/^\d+\.\d+\.\d+(-[\w.-]+)?$/.test(mode)) {
    return mode
  }

  const match = version.match(/^(\d+)\.(\d+)\.(\d+)(-.+)?$/)
  if (!match) {
    throw new Error(`Unsupported current version: ${version}`)
  }

  const major = Number(match[1])
  const minor = Number(match[2])
  const patch = Number(match[3])

  switch (mode) {
    case "patch":
      return `${major}.${minor}.${patch + 1}`
    case "minor":
      return `${major}.${minor + 1}.0`
    case "major":
      return `${major + 1}.0.0`
    default:
      throw new Error(`Unsupported bump mode: ${mode}`)
  }
}
