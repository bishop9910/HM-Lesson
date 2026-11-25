// M5 线上冒烟：资料库 + 批量删除 + 系统后台鉴权 + owner 保护。
// 运行：node scripts/dev/probe-m5b.mjs
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
  const mark = ok ? "PASS" : "FAIL";
  console.log(`${mark} ${name} -> ${status}${extra ? " " + extra : ""}`);
}

async function main() {
  let r = await call("GET", "/healthz");
  record("healthz", r.status, [200]);
  r = await call("GET", "/readyz");
  record("readyz", r.status, [200]);

  r = await call("POST", "/api/v1/auth/login", { phone: OWNER_PHONE, password: OWNER_PASSWORD });
  record("owner login", r.status, [200]);
  const owner = r.data?.token;
  if (!owner) {
    console.log("!! no owner token, skip rest");
    return;
  }
  console.log("owner staff: " + JSON.stringify(r.data.staff));

  r = await call("GET", "/api/v1/system/audit", undefined, owner);
  record("owner -> audit (expect 403)", r.status, [403], JSON.stringify(r.data));
  r = await call("GET", "/api/v1/system/permissions", undefined, owner);
  record("owner -> permissions (expect 403)", r.status, [403]);
  r = await call("POST", "/api/v1/system/query", { sql: "SELECT 1" }, owner);
  record("owner -> console (expect 403)", r.status, [403]);

  r = await call("GET", "/api/v1/staff", undefined, owner);
  record("staff list", r.status, [200]);
  const selfId = r.data?.items?.find((s) => s.phone === OWNER_PHONE)?.id;
  if (selfId) {
    r = await call("PATCH", `/api/v1/staff/${selfId}`, { status: "disabled" }, owner);
    record("disable owner (expect 403 OWNER_PROTECTED)", r.status, [403], JSON.stringify(r.data));
  }

  r = await call("GET", "/api/v1/material-categories", undefined, owner);
  record(
    "material categories",
    r.status,
    [200],
    r.data?.items ? `categories=${r.data.items.length} groups=${JSON.stringify(r.data.groups)}` : "",
  );
  const categoryId = r.data?.items?.[0]?.id;

  r = await call(
    "POST",
    "/api/v1/materials",
    {
      category_id: categoryId,
      title: "线上冒烟-临时资料",
      description: "probe 用，测完即删",
      grade: "",
      files: [{ name: "smoke.pdf", file_id: "cloud://smoke-test-only", size: 1024 }],
    },
    owner,
  );
  record("create material draft", r.status, [201], JSON.stringify(r.data?.material?.status));
  const materialId = r.data?.material?.id;

  if (materialId) {
    r = await call("POST", `/api/v1/materials/${materialId}/publish`, undefined, owner);
    record("publish material", r.status, [200], JSON.stringify(r.data?.material?.status));
    r = await call("POST", `/api/v1/materials/${materialId}/archive`, undefined, owner);
    record("archive material", r.status, [200], JSON.stringify(r.data?.material?.status));
    r = await call("DELETE", `/api/v1/materials/${materialId}`, undefined, owner);
    record("delete material", r.status, [200]);
  }

  r = await call("GET", "/api/v1/material-downloads", undefined, owner);
  record("download logs", r.status, [200]);
  r = await call("GET", "/api/v1/portal/materials");
  record("portal materials without token (expect 401)", r.status, [401]);

  const ids = [];
  for (const name of ["冒烟甲", "冒烟乙"]) {
    r = await call(
      "POST",
      "/api/v1/students",
      { name, grade: "高一", school: "", group_code: "visitor", status: "active", note: "probe" },
      owner,
    );
    if (r.status === 201 || r.status === 200) {
      ids.push(r.data?.student?.id ?? r.data?.id);
    }
  }
  record("create temp students", ids.filter(Boolean).length === 2 ? 201 : 500, [201], `ids=${ids.filter(Boolean).length}`);

  if (ids.filter(Boolean).length) {
    r = await call("POST", "/api/v1/students/batch-delete", { ids: ids.filter(Boolean), force: false }, owner);
    record("batch delete students", r.status, [200], JSON.stringify({ deleted: r.data?.deleted, blocked: r.data?.blocked }));
  }

  r = await call("POST", "/api/v1/guardians/batch-delete", { ids: ["not-exist-id"] }, owner);
  record(
    "batch delete guardians (unknown id -> blocked=1)",
    r.status,
    [200],
    JSON.stringify({ deleted: r.data?.deleted, blocked: r.data?.blocked }),
  );
  r = await call("POST", "/api/v1/guardians/batch-delete", { ids: [] }, owner);
  record("batch delete guardians (empty ids -> 400)", r.status, [400]);

  console.log("\n===== summary =====");
  const fail = results.filter((x) => !x.ok);
  console.log(`total ${results.length}, pass ${results.length - fail.length}, fail ${fail.length}`);
  for (const item of fail) console.log(`FAIL: ${item.name} -> ${item.status} ${item.extra ?? ""}`);
}

main().catch((err) => {
  console.error("probe failed:", err);
  process.exit(1);
});
