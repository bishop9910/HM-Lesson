// 线上冒烟 2：批量开放时段 + 未到课（不扣课时）+ 取消空时段
// 用法：node scripts/dev/probe-m4e.mjs
const BASE = "https://hm-lesson-353410-10-1540142477.sh.run.tcloudbase.com";

function log(name, status, body) {
  console.log(`${name} ${status} ${JSON.stringify(body)}`);
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

// 指定「北京时间」的某个日期 + 时刻，返回 RFC3339
function isoAt(dayOffset, hour, minute = 0) {
  const now = new Date();
  const target = new Date(now.getTime() + dayOffset * 86400000);
  const y = target.getUTCFullYear();
  const m = target.getUTCMonth();
  const d = target.getUTCDate();
  // 北京时间 = UTC+8
  return new Date(Date.UTC(y, m, d, hour - 8, minute, 0)).toISOString();
}

const run = async () => {
  const login = await call("POST", "/api/v1/auth/login", {
    body: { phone: "18852390970", password: "HmTD8M5xg93ib8zKf5" },
  });
  const token = login.body?.token;
  log("staff-login", login.status, { ok: Boolean(token) });
  if (!token) return;

  const students = await call("GET", "/api/v1/students", { token });
  const studentId = students.body?.items?.[0]?.id;

  // 1) 批量开放：明天起两周，周一/周四 14:00-15:00（北京时间）
  const batch = await call("POST", "/api/v1/slots/batch", {
    token,
    body: {
      start_at: isoAt(1, 14, 0),
      end_at: isoAt(1, 15, 0),
      weekdays: [1, 4],
      weeks: 2,
    },
  });
  log("batch-slots", batch.status, batch.body);

  // 2) 未到课：新建一个时段 → 家长预约 → 通过 → 标记未到课
  const slot = await call("POST", "/api/v1/slots", {
    token,
    body: { start_at: isoAt(2, 16, 0), end_at: isoAt(2, 17, 0), note: "未到课冒烟" },
  });
  const slotId = slot.body?.slot?.id;
  log("create-slot", slot.status, { id: slotId });

  const before = await call("GET", `/api/v1/lesson-accounts/${studentId}`, { token });
  const balanceBefore = before.body?.account?.balance_hours;

  const guardianLogin = await call("POST", "/api/v1/auth/guardian/login", {
    body: { phone: "13000000000" },
  });
  const gToken = guardianLogin.body?.token;

  const book = await call("POST", "/api/v1/portal/bookings", {
    token: gToken,
    body: { student_id: studentId, slot_id: slotId, request_note: "未到课测试" },
  });
  const bookingId = book.body?.booking?.id;
  log("portal-book", book.status, { id: bookingId });

  log("approve", ...Object.values(await call("POST", `/api/v1/bookings/${bookingId}/approve`, { token })).slice(0, 2));
  const noShow = await call("POST", `/api/v1/bookings/${bookingId}/no-show`, { token });
  log("no-show", noShow.status, noShow.body);

  const after = await call("GET", `/api/v1/lesson-accounts/${studentId}`, { token });
  log("balance", after.status, {
    before: balanceBefore,
    after: after.body?.account?.balance_hours,
    unchanged: balanceBefore === after.body?.account?.balance_hours,
  });

  // 3) 取消一个空时段（DELETE 只能删没有预约的）
  const slots = await call("GET", "/api/v1/slots", { token });
  const open = (slots.body?.items ?? []).filter((item) => item.status === "open");
  log("open-slots", slots.status, { count: open.length });
  if (open.length > 0) {
    const del = await call("DELETE", `/api/v1/slots/${open[0].id}`, { token });
    log("delete-open-slot", del.status, del.body);
  }
  const locked = (slots.body?.items ?? []).filter((item) => item.status === "locked");
  if (locked.length > 0) {
    const delLocked = await call("DELETE", `/api/v1/slots/${locked[0].id}`, { token });
    log("delete-locked-slot", delLocked.status, delLocked.body);
  }
};

run().catch((err) => {
  console.error("PROBE_FAILED", err);
  process.exitCode = 1;
});
