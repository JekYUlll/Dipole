import exec from "k6/execution";

const scenarios = {};
for (const [index, name] of ["direct_msg", "concurrent", "group_blast"].entries()) {
  scenarios[name] = {
    executor: "per-vu-iterations",
    vus: 3,
    iterations: 1,
    maxDuration: "5s",
    startTime: `${index * 6}s`,
    exec: "probe",
    env: { PROBE_SCENARIO: name },
  };
}

export const options = { scenarios };

export function probe() {
  console.log(JSON.stringify({
    scenario: __ENV.PROBE_SCENARIO,
    index: exec.scenario.iterationInInstance,
    vu: __VU,
  }));
}
