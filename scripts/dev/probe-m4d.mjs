// 线上冒烟：M4 预约 + 课时全链路（教师端写入 + 家长端预约）
// 用法：node scripts/dev/probe-m4d.mjs
const BASE = "https://hm-lesson-353410-10-1540142477.sh.run.tcloudbase.com";

function requirePassword() {
  const pw = process.env.HM_PROBE_PASSWORD;
  if (!pw) {
    console.error("缺少 HM_PROBE_PASSWORD：请用环境变量传测试账号密码，不要把密码写进仓库。");
    process.exit(2);
  }
  return pw;
}


function log(name, status, body) {
  const text = typeof body === "string" ? body : JSON.stringify(body);
  console.log(`${name} ${status} ${text}`);
}

async function call(method, path, { token, body } = {}) {
  const headers = { "Content-Type": "application/json" };
  if (token) headers["X-HM-Token"] = token;
  const res = await fetch(BASE + path, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(20000),
  });
  const text = await res.text();
  let parsed;
  try {
    parsed = JSON.parse(text);
  } catch {
    parsed = text;
  }
  return { status: res.status, body: parsed };
}

function iso(hourUtc, dayOffset) {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() + dayOffset);
  d.setUTCHours(hourUtc, 0, 0, 0);
  return d.toISOString();
}

const run = async () => {
  const health = await call("GET", "/healthz");
  log("healthz", health.status, health.body);

  const login = await call("POST", "/api/v1/auth/login", {
    body: { phone: process.env.HM_PROBE_PHONE ?? "18852390970", password: requirePassword() },
  });
  const token = login.body?.token;
  log("staff-login", login.status, token ? { ok: true } : login.body);
  if (!token) return;

  const students = await call("GET", "/api/v1/students", { token });
  const studentId = students.body?.items?.[0]?.id;
  log("students", students.status, { count: students.body?.items?.length, studentId });

  // 1) 课时充值（append-only 流水）
  const purchase = await call("POST", `/api/v1/lesson-accounts/${studentId}/purchase`, {
    token,
    body: { hours: 10, reason: "线上冒烟" },
  });
  log("purchase", purchase.status, purchase.body);

  const account = await call("GET", `/api/v1/lesson-accounts/${studentId}`, { token });
  log("lesson-account", account.status, account.body?.account);

  // 2) 开放一个时段
  const slot = await call("POST", "/api/v1/slots", {
    token,
    body: { start_at: iso(2, 1), end_at: iso(3, 1), note: "冒烟测试" },
  });
  log("create-slot", slot.status, slot.body);
  const slotId = slot.body?.slot?.id;

  // 3) 参数校验分支
  const badSlot = await call("POST", "/api/v1/slots", {
    token,
    body: { start_at: iso(4, 1), end_at: iso(4, 1) },
  });
  log("bad-slot", badSlot.status, badSlot.body);

  // 4) 批量开放：下周一、三 10:00
  const batch = await call("POST", "/api/v1/slots/batch", {
    token,
    body: { start_time: "10:00", end_time: "11:00", weekdays: [1, 3], weeks: 2 },
  });
  log("batch-slots", batch.status, batch.body);

  // 5) 家长登录（历史测试家长手机号）
  const guardianLogin = await call("POST", "/api/v1/auth/guardian/login", {
    body: { phone: "13000000000" },
  });
  log("guardian-login", guardianLogin.status, guardianLogin.body);
  const gToken = guardianLogin.body?.token;

  if (gToken) {
    const portalSlots = await call("GET", "/api/v1/portal/slots", { token: gToken });
    log("portal-slots", portalSlots.status, {
      count: portalSlots.body?.items?.length,
      first: portalSlots.body?.items?.[0],
    });

    const book = await call("POST", "/api/v1/portal/bookings", {
      token: gToken,
      body: { student_id: studentId, slot_id: slotId, request_note: "冒烟：想练受力分析" },
    });
    log("portal-book", book.status, book.body);
    const bookingId = book.body?.booking?.id;

    // 同一时段重复提交应当 409
    const dup = await call("POST", "/api/v1/portal/bookings", {
      token: gToken,
      body: { student_id: studentId, slot_id: slotId },
    });
    log("duplicate-book", dup.status, dup.body);

    const approve = await call("POST", `/api/v1/bookings/${bookingId}/approve`, { token });
    log("approve", approve.status, approve.body);

    const complete = await call("POST", `/api/v1/bookings/${bookingId}/complete`, { token });
    log("complete", complete.status, complete.body);

    const again = await call("POST", `/api/v1/bookings/${bookingId}/complete`, { token });
    log("complete-idempotent", again.status, again.body);

    const after = await call("GET", `/api/v1/lesson-accounts/${studentId}`, { token });
    log("account-after", after.status, after.body?.account);
    log("ledger-after", after.status, after.body?.ledger?.slice(0, 3));
  }

  const slots = await call("GET", "/api/v1/slots", { token });
  log("slots", slots.status, { count: slots.body?.items?.length });
  for (const item of slots.body?.items ?? []) {
    if (item.status === "open") {
      const res = await call("DELETE", `/api/v1/slots/${item.id}`, { token });
      log("cancel-slot", res.status, { id: item.id, status: res.body?.ok });
    }
  }
};

run().catch((err) => {
  console.error("PROBE_FAILED", err);
  process.exitCode = 1;
});
