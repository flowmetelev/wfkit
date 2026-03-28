#!/usr/bin/env node

import process from "node:process"

const protectedBranch = "refs/heads/main"
const stdin = await readStdin()
const updates = stdin
  .split(/\r?\n/)
  .map(line => line.trim())
  .filter(Boolean)

for (const line of updates) {
  const parts = line.split(/\s+/)
  if (parts.length < 4) {
    continue
  }

  const remoteRef = parts[3]
  if (remoteRef === protectedBranch) {
    console.error(
      "push-guard: direct pushes to main are blocked in this repository. Push a branch and open a pull request instead.",
    )
    process.exit(1)
  }
}

process.exit(0)

function readStdin() {
  return new Promise(resolve => {
    let data = ""
    process.stdin.setEncoding("utf8")
    process.stdin.on("data", chunk => {
      data += chunk
    })
    process.stdin.on("end", () => resolve(data))
  })
}
