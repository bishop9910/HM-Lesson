// v1.3.6 线上冒烟：版本号 + M6 通知 + M7 班课 + M9 看板 + M8 门户聚合。
// 运行：$env:HM_PROBE_PASSWORD="..."; node scripts/dev/probe-m6.mjs
const BASE = "https://hm-lesson-353410-10-1540142477.sh.run.tcloudbase.com";

function requirePassword() {
  const pw = process.env.HM_PROBE_PASSWORD;
  if (!pw) {
    console.error("缺少 HM_PROBE_PASSWORD：请用环境变量传测试账号密码，不要把密码写进仓库。");
    process.exit(2);
  }
  return pw;
}

const OWNER_PHONE = "18852390970";
const OWNER_PASSWORD = requirePassword();
const results = [];

async function call(method, path, body, token) {
  const headers = { "Content-Type": "application/json" };
  if (token) headers["X-HM-Token"] = token;
  const res = await fetch(BASE + path, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
    signal: AbortSignal.timeout(20000),
  });
  const text = await res.text();
  let data = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = text.slice(0, 300);
  }
  return { status: res.status, data };
}

function record(name, status, expect, extra) {
  const ok = expect.includes(status);
  results.push({ name, status, ok, extra });
  console.log(`${ok ? "PASS" : "FAIL"} ${name} -> ${status}${extra ? " " + extra : ""}`);
}

async function main() {
  let r = await call("GET", "/healthz");
  const version = r.data?.version;
  record("healthz", r.status, [200], `version=${version}`);
  record("healthz version is 1.3.6", version === "1.3.6" ? 200 : 500, [200], `got ${version}`);

  r = await call("GET", "/readyz");
  record("readyz", r.status, [200]);

  r = await call("POST", "/api/v1/auth/login", { phone: OWNER_PHONE, password: OWNER_PASSWORD });
  record("owner login", r.status, [200]);
  const owner = r.data?.token;
  if (!owner) {
    console.log("!! no owner token, skip rest");
    return;
  }

  // ---- M9 看板 ----
  r = await call("GET", "/api/v1/dashboard/board", undefined, owner);
  record(
    "dashboard/board",
    r.status,
    [200],
    r.data?.board ? `groups=${r.data.board.groups?.length} new=${r.data.board.new_students}` : "",
  );
  r = await call("GET", "/api/v1/dashboard/board?from=bad", undefined, owner);
  record("dashboard/board bad range -> 400", r.status, [400]);

  // ---- M6 通知 ----
  r = await call("GET", "/api/v1/notices", undefined, owner);
  record("list notices", r.status, [200], `items=${r.data?.items?.length ?? 0}`);

  r = await call(
    "POST",
    "/api/v1/notices",
    { title: "冒烟-临时通知", body: "probe 用，测完即删", audience_type: "all", audience_value: "", need_receipt: true, files: [] },
    owner,
  );
  record("create notice draft", r.status, [201], JSON.stringify(r.data?.notice?.status));
  const noticeId = r.data?.notice?.id;

  if (noticeId) {
    r = await call("GET", `/api/v1/notices/${noticeId}`, undefined, owner);
    record("get notice", r.status, [200]);
    r = await call("PATCH", `/api/v1/notices/${noticeId}`, { title: "冒烟-临时通知(改)", body: "probe", need_receipt: false }, owner);
    record("patch notice", r.status, [200]);
    // 线上没有家长时发送应因受众为空被拒（400 NOTICE_NO_AUDIENCE），而不是 500。
    r = await call("POST", `/api/v1/notices/${noticeId}/send`, undefined, owner);
    record("send notice (no audience -> 400)", r.status, [200, 400], JSON.stringify(r.data?.reason ?? ""));
    r = await call("DELETE", `/api/v1/notices/${noticeId}`, undefined, owner);
    record("delete notice draft", r.status, [200]);
  }

  r = await call("GET", "/api/v1/portal/notices");
  record("portal notices without token -> 401", r.status, [401]);

  // ---- M7 班课 ----
  r = await call("POST", "/api/v1/students", { name: "冒烟-班课学员", grade: "高一", school: "", group_code: "class_active", status: "active", note: "probe" }, owner);
  const studentId = r.data?.student?.id ?? r.data?.id;
  record("create temp student", r.status, [200, 201], `id=${studentId}`);

  r = await call("GET", "/api/v1/classes", undefined, owner);
  record("list classes", r.status, [200], `items=${r.data?.items?.length ?? 0}`);

  r = await call(
    "POST",
    "/api/v1/classes",
    { name: "冒烟-临时班（可忽略）", subject: "physics", start_date: "", end_date: "", schedule_note: "probe", student_ids: [] },
    owner,
  );
  record("create class", r.status, [201], JSON.stringify(r.data?.class?.status));
  const classId = r.data?.class?.id;

  if (classId && studentId) {
    r = await call("POST", `/api/v1/classes/${classId}/members`, { student_ids: [studentId] }, owner);
    record("add class member", r.status, [200], JSON.stringify({ added: r.data?.added }));
    r = await call("POST", `/api/v1/classes/${classId}/sessions`, { session_date: "2026-11-25", content: "probe 课次", attendance: { [studentId]: "present" } }, owner);
    record("create class session", r.status, [201]);
    const sessionId = r.data?.session?.id;
    r = await call("GET", `/api/v1/classes/${classId}`, undefined, owner);
    record("get class detail", r.status, [200], `members=${r.data?.members?.length} sessions=${r.data?.sessions?.length}`);
    if (sessionId) {
      r = await call("DELETE", `/api/v1/classes/${classId}/sessions/${sessionId}`, undefined, owner);
      record("delete class session", r.status, [200]);
    }
    r = await call("DELETE", `/api/v1/classes/${classId}/members/${studentId}`, undefined, owner);
    record("remove class member", r.status, [200]);
    r = await call("POST", `/api/v1/classes/${classId}/archive`, undefined, owner);
    record("archive class", r.status, [200], JSON.stringify(r.data?.class?.status));
  }

  if (studentId) {
    r = await call("DELETE", `/api/v1/students/${studentId}`, undefined, owner);
    record("delete temp student", r.status, [200]);
  }

  r = await call("GET", "/api/v1/portal/home");
  record("portal home without token -> 401", r.status, [401]);

  console.log("\n===== summary =====");
  const fail = results.filter((x) => !x.ok);
  console.log(`total ${results.length}, pass ${results.length - fail.length}, fail ${fail.length}`);
  for (const item of fail) console.log(`FAIL: ${item.name} -> ${item.status} ${item.extra ?? ""}`);
  if (fail.length) process.exitCode = 1;
}

main().catch((err) => {
  console.error("probe failed:", err);
  process.exit(1);
});
