// 通知回执全链路 + 资料可见性：owner 发通知 → 家长看到 → 家长确认 → 老师看到已确认 → 导出 xlsx
// 密码一律从环境变量读取：HM_PROBE_PASSWORD（owner）、HM_PROBE_GUARDIAN_PASSWORD（家长）
const BASE = "https://hm-lesson-353410-10-1540142477.sh.run.tcloudbase.com";
const OWNER_PHONE = process.env.HM_PROBE_OWNER_PHONE || "18852390970";
const GUARDIAN_PHONE = process.env.HM_PROBE_GUARDIAN_PHONE || "13000000000";

function requireEnv(name) {
  const value = process.env[name];
  if (!value) {
    console.error(`缺少环境变量 ${name}`);
    process.exit(2);
  }
  return value;
}

const ownerPassword = requireEnv("HM_PROBE_PASSWORD");
const guardianPassword = requireEnv("HM_PROBE_GUARDIAN_PASSWORD");

let pass = 0;
let fail = 0;

async function call(path, { method = "GET", token, body, raw = false } = {}) {
  const headers = {};
  if (token) headers["X-HM-Token"] = token;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const res = await fetch(BASE + path, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(20000),
  });
  if (raw) return res;
  const text = await res.text();
  let data = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = text;
  }
  return { status: res.status, data };
}

function check(name, ok, detail) {
  if (ok) {
    pass += 1;
    console.log(`PASS ${name}${detail ? ` -> ${detail}` : ""}`);
  } else {
    fail += 1;
    console.log(`FAIL ${name}${detail ? ` -> ${detail}` : ""}`);
  }
}

const stamp = new Date().toISOString().slice(0, 16).replace("T", " ");

const owner = await call("/api/v1/auth/staff/login", {
  method: "POST",
  body: { phone: OWNER_PHONE, password: ownerPassword },
});
check("owner login", owner.status === 200, `${owner.status}`);
const ownerToken = owner.data?.token;

const students = await call("/api/v1/students", { token: ownerToken });
check("list students", students.status === 200, `${students.status} items=${students.data?.items?.length ?? "?"}`);
const student = (students.data?.items ?? []).find((s) => s.name === "test") ?? students.data?.items?.[0];
console.log(`      student: ${student?.id} ${student?.name}`);

const created = await call("/api/v1/notices", {
  method: "POST",
  token: ownerToken,
  body: {
    title: `回执冒烟 ${stamp}`,
    body: "这是一条用于验证回执链路的测试通知，家长点「我已确认」即可。",
    audience_type: "students",
    audience_value: student.id,
    need_receipt: true,
  },
});
check("create notice (need receipt)", created.status === 201, `${created.status}`);
const noticeID = created.data?.notice?.id;

const sent = await call(`/api/v1/notices/${noticeID}/send`, { method: "POST", token: ownerToken });
check("send notice", sent.status === 200, `${sent.status} receipts=${sent.data?.receipts?.length ?? "?"}`);

const guardian = await call("/api/v1/auth/guardian/login", {
  method: "POST",
  body: { phone: GUARDIAN_PHONE, password: guardianPassword },
});
check("guardian login", guardian.status === 200, `${guardian.status}`);
const guardianToken = guardian.data?.token;
check("guardian has students", (guardian.data?.students?.length ?? 0) > 0, `${guardian.data?.students?.length ?? 0}`);

const home = await call("/api/v1/portal/home", { token: guardianToken });
check(
  "portal home pending_receipts",
  home.status === 200 && (home.data?.pending_receipts ?? 0) >= 1,
  `${home.status} pending_receipts=${home.data?.pending_receipts}`,
);

const portalNotices = await call("/api/v1/portal/notices", { token: guardianToken });
const mine = (portalNotices.data?.items ?? []).find((n) => n.id === noticeID);
check(
  "portal notices contains new notice",
  portalNotices.status === 200 && !!mine,
  `${portalNotices.status} my_receipt_status=${mine?.my_receipt_status}`,
);

const confirmed = await call(`/api/v1/portal/notices/${noticeID}/confirm`, { method: "POST", token: guardianToken });
check("guardian confirm receipt", confirmed.status === 200, `${confirmed.status}`);

const receipts = await call(`/api/v1/notices/${noticeID}/receipts`, { token: ownerToken });
const receipt = (receipts.data?.items ?? []).find((r) => r.phone === GUARDIAN_PHONE);
check(
  "owner sees confirmed receipt",
  receipts.status === 200 && receipt?.status === "confirmed",
  `${receipts.status} status=${receipt?.status} confirmed_at=${receipt?.confirmed_at}`,
);

const xlsx = await call(`/api/v1/notices/${noticeID}/receipts.xlsx`, { token: ownerToken, raw: true });
const xlsxBuf = Buffer.from(await xlsx.arrayBuffer());
check(
  "export receipts xlsx",
  xlsx.status === 200 && xlsxBuf.length > 1000 && xlsxBuf.subarray(0, 2).toString() === "PK",
  `${xlsx.status} ${xlsxBuf.length}B type=${xlsx.headers.get("content-type")}`,
);

const categories = await call("/api/v1/material-categories", { token: ownerToken });
const handout = (categories.data?.items ?? []).find((c) => c.code === "class_handout") ?? categories.data?.items?.[0];
const openCategory = (categories.data?.items ?? []).find((c) => c.visibility?.visitor === true);
check(
  "list material categories",
  categories.status === 200 && !!handout && !!openCategory,
  `${categories.status} handout=${handout?.code} visitorOpen=${openCategory?.code}`,
);

// 没有附件的资料不能发布（家长端只能看到标题、下载不到东西）
const emptyMaterial = await call("/api/v1/materials", {
  method: "POST",
  token: ownerToken,
  body: { title: `空资料 ${stamp}`, description: "", category_id: openCategory?.id, grade: "", files: [] },
});
const emptyID = emptyMaterial.data?.material?.id;
const emptyPublish = await call(`/api/v1/materials/${emptyID}/publish`, { method: "POST", token: ownerToken });
check(
  "publish without files rejected",
  emptyPublish.status === 400 && emptyPublish.data?.reason === "MATERIAL_NO_FILES",
  `${emptyPublish.status} ${emptyPublish.data?.reason}`,
);
await call(`/api/v1/materials/${emptyID}`, { method: "DELETE", token: ownerToken });

const probeFile = { name: "冒烟讲义.pdf", file_id: "cloud://hm-lesson-test/probe/notice-probe.pdf", size: 1024 };

// 负向：班课讲义默认只对 class_active 可见，家长绑定的是 visitor 学员 → 应该看不到
const hidden = await call("/api/v1/materials", {
  method: "POST",
  token: ownerToken,
  body: {
    title: `不该可见 ${stamp}`,
    description: "分组不可见性验证",
    category_id: handout?.id,
    grade: "",
    files: [probeFile],
  },
});
check("create material (no files)", hidden.status === 201, `${hidden.status} ${hidden.data?.reason ?? ""}`);
const hiddenID = hidden.data?.material?.id;
const hiddenPublished = await call(`/api/v1/materials/${hiddenID}/publish`, { method: "POST", token: ownerToken });
check("publish hidden material", hiddenPublished.status === 200, `${hiddenPublished.status}`);

const portalMaterials = await call("/api/v1/portal/materials", { token: guardianToken });
const hiddenVisible = (portalMaterials.data?.items ?? []).some((m) => m.id === hiddenID);
check(
  "portal hides material of other group",
  portalMaterials.status === 200 && hiddenVisible === false,
  `${portalMaterials.status} leaked=${hiddenVisible}`,
);

// 正向：换成本组可见的分类（visibility.visitor=true）→ 家长能看到并下载，且留下下载日志
const material = await call("/api/v1/materials", {
  method: "POST",
  token: ownerToken,
  body: {
    title: `资料冒烟 ${stamp}`,
    description: "验证家长端可见性与下载日志",
    category_id: openCategory?.id,
    grade: "",
    files: [probeFile],
  },
});
check("create visible material", material.status === 201, `${material.status} ${material.data?.reason ?? ""}`);
const materialID = material.data?.material?.id;

const published = await call(`/api/v1/materials/${materialID}/publish`, { method: "POST", token: ownerToken });
check("publish visible material", published.status === 200, `${published.status}`);

const portalAgain = await call("/api/v1/portal/materials", { token: guardianToken });
const visible = (portalAgain.data?.items ?? []).find((m) => m.id === materialID);
check("portal sees visible material", visible !== undefined, `${portalAgain.status} visible=${visible !== undefined}`);

const download = await call(`/api/v1/portal/materials/${materialID}/download`, { method: "POST", token: guardianToken });
check("portal material download", download.status === 200, `${download.status}`);

const logs = await call("/api/v1/material-downloads", { token: ownerToken });
check(
  "download logs recorded",
  logs.status === 200 && (logs.data?.items ?? []).some((l) => l.material_id === materialID),
  `${logs.status} items=${logs.data?.items?.length}`,
);

const removed = await call(`/api/v1/materials/${materialID}`, { method: "DELETE", token: ownerToken });
check("delete material", removed.status === 200, `${removed.status}`);
const removedHidden = await call(`/api/v1/materials/${hiddenID}`, { method: "DELETE", token: ownerToken });
check("delete hidden material", removedHidden.status === 200, `${removedHidden.status}`);

console.log("\n===== summary =====");
console.log(`total ${pass + fail}, pass ${pass}, fail ${fail}`);
console.log(`notice left in DB: ${noticeID}（已发送的通知不可删除，测试环境保留即可）`);
process.exit(fail === 0 ? 0 : 1);
