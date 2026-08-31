import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

const script = fs.readFileSync(new URL("./docker-build-microservice-images.sh", import.meta.url), "utf8");
const dockerfile = fs.readFileSync(new URL("../deploy/images/go-service.Dockerfile", import.meta.url), "utf8");

test("Go microservice images stage only the target binary as the Docker context", () => {
  assert.match(script, /context_dir="\$\{root_dir\}\/dist"/);
  assert.match(script, /build_context=\$\(mktemp -d -t dipole-microservice-image\.XXXXXX\)/);
  assert.match(script, /install -m 755 "\$\{source_binary\}" "\$\{build_context\}\/\$\{binary\}"/);
  assert.match(script, /--file deploy\/images\/go-service\.Dockerfile[\s\S]*?"\$\{build_context\}"/);
  assert.doesNotMatch(script, /--file deploy\/images\/go-service\.Dockerfile[\s\S]*?"\$\{context_dir\}"/);
  assert.match(dockerfile, /COPY \$\{DIPOLE_BINARY\} \/app\/service/);
  assert.doesNotMatch(dockerfile, /COPY dist\/\$\{DIPOLE_BINARY\}/);
});

test("microservice image builds include the TypeScript Agent Runtime at the same revision", () => {
  assert.match(script, /agent_image=\$\{DIPOLE_AGENT_IMAGE:-dipole-agent:latest\}/);
  assert.match(script, /--file services\/agent-runtime\/Dockerfile[\s\S]*?services\/agent-runtime/);
  assert.match(script, /--build-arg "DIPOLE_VCS_REVISION=\$\{revision\}"/);
  assert.match(script, /--build-arg "DIPOLE_VCS_DIRTY=\$\{dirty\}"/);
});
