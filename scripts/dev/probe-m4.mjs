const BASE = "https://hm-lesson-353410-10-1540142477.sh.run.tcloudbase.com";
const OWNER = { phone: "18852390970", password: "HmTD8M5xg93ib8zKf5" };

const log = (...parts) => console.log(...parts);

async function req(path, init) {
  const res = await fetch(BASE + path, { ...init, signal: AbortSignal.timeout(20000) });
  const text = await res.text();
  return { status: res.status, text };
}

function show(label, r) {
  log(label, r.status, r.text.slice(0, 700));
}

const main = async () => {
  show("healthz", await req("/healthz"));
  show("readyz", await req("/readyz"));

  const login = await req("/api/v1/auth/login", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(OWNER),
  });
  show("staff-login", login);
  const token = login.status === 200 ? JSON.parse(login.text).token : "";
  const H = { headers: { "X-HM-Token": token } };

  for (const p of [
    "/api/v1/dashboard/summary",
    "/api/v1/slots",
    "/api/v1/bookings",
    "/api/v1/calendar/bookings",
    "/api/v1/lesson-accounts",
    "/api/v1/lesson-ledger",
  ]) {
    show(p, await req(p, H));
  }
};

main().catch((e) => log("ERR", e.message));
