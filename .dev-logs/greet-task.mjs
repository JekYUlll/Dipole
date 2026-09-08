const owner = process.env.OWNER;
const crid = process.env.CRID;
const goal = "你好！你能帮我做些什么？";
const res = await fetch("http://127.0.0.1:8091/internal/v1/agent/tasks", {
  method: "POST",
  headers: {
    "content-type": "application/json",
    "x-dipole-caller-service": "dipole-gateway",
    "x-dipole-service-token": process.env.DIPOLE_AGENT_CONTROL_SECRET,
    "x-dipole-principal-user-id": owner,
    "x-request-id": `REQ-${crid}`,
    "x-trace-id": `TRACE-${crid}`
  },
  body: JSON.stringify({ clientRequestId: crid, goal })
});
const body = await res.json();
console.log("STATUS", res.status);
console.log("TASK", body.taskId || JSON.stringify(body));
console.log("GOAL", goal);
