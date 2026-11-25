const BASE = "https://hm-lesson-353410-10-1540142477.sh.run.tcloudbase.com";
const OWNER = { phone: "18852390970", password: "HmTD8M5xg93ib8zKf5" };

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

  show("tasks", await req("/api/v1/tasks", H));
  show("reviews", await req("/api/v1/reviews", H));
  show("students", await req("/api/v1/students", H));
  // 旧接口若正常、新接口 500，说明老表可读，问题集中在新表/新 SQL
  show(
    "batch-slots",
    await req("/api/v1/slots/batch", {
      method: "POST",
      headers: H.headers,
      body: JSON.stringify({ start_at: "2026-11-27T02:00:00Z", end_at: "2026-11-27T03:00:00Z", weekdays: [5], weeks: 1 }),
    }),
  );
};

main().catch((e) => console.log("ERR", e.message));
