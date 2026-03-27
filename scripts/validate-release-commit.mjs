#!/usr/bin/env node

import fs from "node:fs"
import process from "node:process"
import { execFileSync } from "node:child_process"

const commitMsgPath = process.argv[2]

if (!commitMsgPath) {
  console.error("Usage: node scripts/validate-release-commit.mjs <commit-msg-file>")
  process.exit(1)
}

const commitMessage = fs.readFileSync(commitMsgPath, "utf8")
const firstLine = commitMessage.split(/\r?\n/, 1)[0].trim()
const releaseMatch = firstLine.match(/^chore: release v(.+)$/)

if (!releaseMatch) {
  process.exit(0)
}

const expectedVersion = releaseMatch[1]
const stagedFiles = execGit(["diff", "--cached", "--name-only", "--diff-filter=ACMR"])
  .split(/\r?\n/)
  .filter(Boolean)

if (!stagedFiles.includes("npm/package.json")) {
  fail(
    `Release commit "${firstLine}" must include a staged change to npm/package.json.`,
  )
}

let stagedPackageJsonText
try {
  stagedPackageJsonText = execGit(["show", ":npm/package.json"])
} catch {
  fail("Could not read staged npm/package.json from the git index.")
}

let stagedPackageJson
try {
  stagedPackageJson = JSON.parse(stagedPackageJsonText)
} catch {
  fail("Staged npm/package.json is not valid JSON.")
}

if (stagedPackageJson.version !== expectedVersion) {
  fail(
    `Release commit message expects version ${expectedVersion}, but staged npm/package.json has version ${stagedPackageJson.version}.`,
  )
}

function execGit(args) {
  return execFileSync("git", args, { encoding: "utf8" }).trim()
}

function fail(message) {
  console.error(`release-guard: ${message}`)
  process.exit(1)
}
