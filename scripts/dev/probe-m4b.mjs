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

async function req(path, init) {
  const res = await fetch(BASE + path, { ...init, signal: AbortSignal.timeout(20000) });
  const text = await res.text();
  return { status: res.status, text };
}
const show = (label, r) => console.log(label, r.status, r.text.slice(0, 400));

const main = async () => {
  const login = await req("/api/v1/auth/login", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(OWNER),
  });
  const token = JSON.parse(login.text).token;
  const H = { headers: { "X-HM-Token": token, "content-type": "application/json" } };

  // 验证参数校验分支是否可达（应 400 而不是 500）
  show("slots?from=bad", await req("/api/v1/slots?from=bad", H));
  show("slots?from=2026-11-25T00:00:00Z", await req("/api/v1/slots?from=2026-11-25T00:00:00Z", H));
  show(
    "create-slot",
    await req("/api/v1/slots", {
      method: "POST",
      headers: H.headers,
      body: JSON.stringify({ start_at: "2026-11-27T02:00:00Z", end_at: "2026-11-27T03:00:00Z", note: "probe" }),
    }),
  );
};

main().catch((e) => console.log("ERR", e.message));
