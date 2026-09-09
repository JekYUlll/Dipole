import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

const script = fs.readFileSync(new URL("./docker-build-microservice-images.sh", import.meta.url), "utf8");
const dockerfile = fs.readFileSync(new URL("../deploy/images/go-service.Dockerfile", import.meta.url), "utf8");

test("Go microservice images stage only the target binary as the Docker context", () => {
  assert.match(script, /context_dir="\$\{root_dir\}\/dist"/);
  assert.match(script, /for service_binary in "\$\{services\[@\]\}"; do[\s\S]*?\(\n    build_context=\$\(mktemp -d -t dipole-microservice-image\.XXXXXX\)/);
  assert.match(script, /trap 'rm -rf "\$\{build_context\}"' EXIT/);
  assert.match(script, /install -m 755 "\$\{source_binary\}" "\$\{build_context\}\/\$\{binary\}"/);
  assert.match(script, /--file deploy\/images\/go-service\.Dockerfile[\s\S]*?"\$\{build_context\}"/);
  assert.doesNotMatch(script, /--file deploy\/images\/go-service\.Dockerfile[\s\S]*?"\$\{context_dir\}"/);
  assert.doesNotMatch(script, /context_dir="\$\{root_dir\}\/dist"\n\s*build_context=/);
  assert.match(dockerfile, /COPY \$\{DIPOLE_BINARY\} \/app\/service/);
  assert.doesNotMatch(dockerfile, /COPY dist\/\$\{DIPOLE_BINARY\}/);
});

test("Go microservice images cache base dependencies before service-specific build arguments", () => {
  const dependencyLayer = dockerfile.indexOf("RUN apk add --no-cache ca-certificates tzdata");
  const binaryArgument = dockerfile.indexOf("ARG DIPOLE_BINARY");
  const provenanceLayer = dockerfile.indexOf("LABEL org.opencontainers.image.revision");

  assert.ok(dependencyLayer >= 0, "base dependency layer must exist");
  assert.ok(binaryArgument > dependencyLayer, "service binary selection must not invalidate the shared dependency layer");
  assert.ok(provenanceLayer > binaryArgument, "provenance labels must follow service-specific build arguments");
});

test("immutable release builds accept explicit provenance without a Git worktree", () => {
  assert.match(script, /revision=\$\{DIPOLE_BUILD_REVISION:-\}/);
  assert.match(script, /if \[\[ -z "\$\{revision\}" \]\]; then\n  revision=\$\(git rev-parse HEAD\)/);
  assert.match(script, /DIPOLE_BUILD_REVISION must be a full lowercase Git SHA/);
  assert.match(script, /dirty=\$\{DIPOLE_BUILD_DIRTY:-\}/);
  assert.match(script, /DIPOLE_BUILD_DIRTY must be true or false/);
});

test("microservice image builds include the TypeScript Agent Runtime at the same revision", () => {
  assert.match(script, /agent_service="agent"/);
  assert.match(script, /\[\[ "\$\{requested_service\}" != "\$\{agent_service\}" \]\] \|\| found=true/);
  assert.match(script, /agent_image=\$\{DIPOLE_AGENT_IMAGE:-dipole-agent:latest\}/);
  assert.match(script, /--file services\/agent-runtime\/Dockerfile[\s\S]*?services\/agent-runtime/);
  assert.match(script, /--build-arg "DIPOLE_VCS_REVISION=\$\{revision\}"/);
  assert.match(script, /--build-arg "DIPOLE_VCS_DIRTY=\$\{dirty\}"/);
});

test("microservice selection accepts documented space- and comma-delimited service lists", () => {
  assert.ok(script.includes('selected_services=",${selected_services// /,},"'));
  assert.ok(script.includes('normalized_selected_services=${selected_services//,/ }'));
  assert.match(script, /if \[\[ -z "\$\{selected_services\}" \|\| "\$\{selected_services\}" == \*",\$\{agent_service\},"\* \]\]; then/);
});
