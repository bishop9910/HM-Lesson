const BASE = "https://hm-lesson-353410-10-1540142477.sh.run.tcloudbase.com";

function requirePassword() {
  const pw = process.env.HM_PROBE_PASSWORD;
  if (!pw) {
    console.error("缺少 HM_PROBE_PASSWORD：请用环境变量传测试账号密码，不要把密码写进仓库。");
    process.exit(2);
  }
  return pw;
}

const OWNER = { phone: process.env.HM_PROBE_PHONE ?? "18852390970", password: requirePassword() };

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
