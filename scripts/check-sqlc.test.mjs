import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { chmodSync, cpSync, mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = join(fileURLToPath(new URL("..", import.meta.url)));

function createFixture() {
  const fixture = mkdtempSync(join(tmpdir(), "dipole-sqlc-gate-"));
  const scripts = join(fixture, "scripts");
  const generated = join(fixture, "internal", "platform", "mysql", "generated");
  const fakeSqlc = join(fixture, "sqlc");

  mkdirSync(scripts, { recursive: true });
  mkdirSync(generated, { recursive: true });
  cpSync(join(root, "scripts", "check-sqlc.sh"), join(scripts, "check-sqlc.sh"));
  cpSync(join(root, "scripts", "sqlc.sh"), join(scripts, "sqlc.sh"));
  writeFileSync(join(fixture, "go.mod"), "module fixture\n\ngo 1.27\n");
  writeFileSync(join(fixture, "go.sum"), "");
  writeFileSync(join(fakeSqlc), "#!/usr/bin/env bash\nif [[ \"${1:-}\" == version ]]; then echo v1.31.1; fi\n");
  chmodSync(fakeSqlc, 0o755);
  execFileSync("git", ["init", "-q"], { cwd: fixture });
  execFileSync("git", ["add", "."], { cwd: fixture });

  return { fixture, fakeSqlc };
}

function runFixture(fixture, fakeSqlc) {
  return execFileSync("bash", ["scripts/check-sqlc.sh"], {
    cwd: fixture,
    env: { ...process.env, SQLC_BIN: fakeSqlc },
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  });
}

function assertRejected(source, expectedMessage) {
  const { fixture, fakeSqlc } = createFixture();
  mkdirSync(join(fixture, "internal", "example"), { recursive: true });
  writeFileSync(join(fixture, "internal", "example", "legacy_test.go"), source);

  assert.throws(
    () => runFixture(fixture, fakeSqlc),
    error => error.status === 1 && `${error.stdout}${error.stderr}`.includes(expectedMessage),
  );
}

test("accepts a SQLC-only repository", () => {
  const { fixture, fakeSqlc } = createFixture();
  assert.doesNotThrow(() => runFixture(fixture, fakeSqlc));
});

test("rejects GORM imports in Go code", () => {
  assertRejected('package example\n\nimport _ "gorm.io/gorm"\n', "GORM or AutoMigrate references remain");
});

test("rejects runtime AutoMigrate calls in Go code", () => {
  assertRejected("package example\n\nfunc migrate(db interface{ AutoMigrate(...any) error }) error { return db.AutoMigrate() }\n", "GORM or AutoMigrate references remain");
});

test("rejects legacy GORM module dependencies", () => {
  const { fixture, fakeSqlc } = createFixture();
  writeFileSync(join(fixture, "go.mod"), "module fixture\n\ngo 1.27\n\nrequire github.com/jinzhu/gorm v1.9.16\n");
  assert.throws(
    () => runFixture(fixture, fakeSqlc),
    error => error.status === 1 && `${error.stdout}${error.stderr}`.includes("GORM module dependencies remain"),
  );
});
