/**
 * Fail on HIGH/CRITICAL npm advisories except IDs in npm-audit-allowlist.json.
 * Each allowlisted GHSA must have a why. Do not use npm audit --force.
 */
import { execSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const allowDoc = JSON.parse(readFileSync(join(here, "npm-audit-allowlist.json"), "utf8"));
const allow = new Set(Object.keys(allowDoc.advisories ?? {}));

let audit;
try {
  execSync("npm audit --json", { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] });
  audit = { vulnerabilities: {} };
} catch (err) {
  const stdout = err.stdout?.toString?.() ?? "";
  audit = JSON.parse(stdout || "{}");
}

function ghsaIds(via) {
  const ids = [];
  for (const item of via ?? []) {
    if (typeof item === "string") {
      continue;
    }
    const url = String(item.url ?? "");
    const m = url.match(/GHSA-[\w-]+/);
    if (m) {
      ids.push(m[0]);
    }
  }
  return ids;
}

const blocking = [];
for (const [name, info] of Object.entries(audit.vulnerabilities ?? {})) {
  if (!["high", "critical"].includes(info.severity)) {
    continue;
  }
  const ids = ghsaIds(info.via);
  if (ids.length === 0) {
    continue;
  }
  const unexplained = ids.filter((id) => !allow.has(id));
  if (unexplained.length > 0) {
    blocking.push({ name, severity: info.severity, ids: unexplained });
  }
}

if (blocking.length) {
  console.error("npm HIGH/CRITICAL not in scripts/npm-audit-allowlist.json:");
  console.error(JSON.stringify(blocking, null, 2));
  process.exit(1);
}

console.log("npm audit high+ clean (allowlisted Next 15 transitives only)");
