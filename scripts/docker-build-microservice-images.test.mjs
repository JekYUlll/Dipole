import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

const script = fs.readFileSync(new URL("./docker-build-microservice-images.sh", import.meta.url), "utf8");
const dockerfile = fs.readFileSync(new URL("../deploy/images/go-service.Dockerfile", import.meta.url), "utf8");

test("Go microservice images use the generated dist directory as context", () => {
  assert.match(script, /context_dir="\$\{ROOT_DIR\}\/dist"/);
  assert.match(script, /--file deploy\/images\/go-service\.Dockerfile[\s\S]*?"\$\{context_dir\}"/);
  assert.match(dockerfile, /COPY \$\{DIPOLE_BINARY\} \/app\/service/);
  assert.doesNotMatch(dockerfile, /COPY dist\/\$\{DIPOLE_BINARY\}/);
});
