import crypto from "node:crypto";

const SECRET = process.env.JWT_SECRET || "dipole-dev-jwt-secret-change-me";
const USER = process.env.USER_UUID || "U9927C817A871D9271CF8";
const ASSISTANT = process.env.ASSISTANT_UUID || "UAI000000000000000001";
const GW = process.env.GW || "gateway:8080";

const b64url = (s) => Buffer.from(s).toString("base64url");
function mintJWT(sub) {
  const now = Math.floor(Date.now() / 1000);
  const h = b64url(JSON.stringify({ alg: "HS256", typ: "JWT" }));
  const p = b64url(JSON.stringify({ sub, iss: "dipole", iat: now, nbf: now, exp: now + 3600, jti: crypto.randomUUID(), token_use: "session" }));
  const sig = crypto.createHmac("sha256", SECRET).update(h + "." + p).digest("base64url");
  return h + "." + p + "." + sig;
}

const token = mintJWT(USER);
const cmid = "e2e-" + crypto.randomUUID().slice(0, 8);
const content = "你好，你能帮我做什么？顺便帮我看看我最近的会话有哪些。";

async function listDirect() {
  const res = await fetch("http://" + GW + "/api/v1/messages/direct/" + ASSISTANT + "?limit=15", { headers: { Authorization: "Bearer " + token } });
  const txt = await res.text();
  let body; try { body = JSON.parse(txt); } catch { body = txt; }
  const arr = (body && body.data) ? body.data : (Array.isArray(body) ? body : []);
  return arr;
}

const before = await listDirect().catch(() => []);
console.log("baseline messages in direct conv:", before.length);

const ws = new WebSocket("ws://" + GW + "/api/v1/ws?token=" + token);
ws.addEventListener("open", () => {
  console.log("WS open -> chat.send to assistant");
  ws.send(JSON.stringify({ type: "chat.send", data: { target_uuid: ASSISTANT, content, client_message_id: cmid } }));
});
ws.addEventListener("message", (ev) => {
  const s = typeof ev.data === "string" ? ev.data : "[binary]";
  console.log("WS<-", s.slice(0, 300));
});
ws.addEventListener("error", (e) => console.log("WS error", (e && e.message) || e));

let tries = 0;
const timer = setInterval(async () => {
  tries++;
  const arr = await listDirect().catch(() => []);
  const assistantMsgs = arr.filter((m) => (m.senderUUID || m.sender_uuid) === ASSISTANT);
  console.log(`poll#${tries} total=${arr.length} assistantMsgs=${assistantMsgs.length}`);
  if (assistantMsgs.length > 0) {
    console.log("=== ASSISTANT REPLIES ===");
    for (const m of assistantMsgs) {
      console.log(`  type=${m.messageType || m.message_type} :: ${String(m.content || "").slice(0, 260)}`);
    }
    clearInterval(timer); try { ws.close(); } catch {} process.exit(0);
  }
  if (tries >= 8) {
    console.log("=== TIMEOUT: no assistant reply; dumping last messages ===");
    for (const m of arr.slice(-6)) console.log(`  [${m.senderUUID || m.sender_uuid}] ${String(m.content || "").slice(0, 160)}`);
    clearInterval(timer); try { ws.close(); } catch {} process.exit(1);
  }
}, 12000);
