import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

const source = fs.readFileSync(new URL("./drain-local-dipole.sh", import.meta.url), "utf8");

test("local drain requires explicit apply and targets only Dipole containers", () => {
  assert.match(source, /mode="\$\{1:---dry-run\}"/);
  assert.match(source, /\$2 ~ \/\^dipole\//);
  assert.match(source, /docker stop "\$id"/);
  assert.doesNotMatch(source, /--volumes|docker rm|docker rmi/);
});
