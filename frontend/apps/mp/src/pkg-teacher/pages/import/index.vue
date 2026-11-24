<template>
  <view class="page">
    <view class="hero">
      <text class="hero-title">导入学员</text>
      <text class="hero-sub">按模板填写 Excel，先预览再导入</text>
    </view>

    <view class="card">
      <button class="btn ghost" :loading="busy" @tap="downloadTemplate">下载模板</button>
      <button class="btn primary" :loading="busy" @tap="chooseFile">选择 Excel 文件</button>
      <text class="hint">支持 .xlsx；单次最多 5000 行、5MB。手机号相同的学员会跳过。</text>
    </view>

    <view v-if="preview" class="card">
      <view class="summary">
        <view class="metric">
          <text class="num">{{ preview.to_create }}</text>
          <text class="label">将新增</text>
        </view>
        <view class="metric">
          <text class="num">{{ preview.to_skip }}</text>
          <text class="label">将跳过</text>
        </view>
        <view class="metric">
          <text class="num">{{ preview.batch.error_count }}</text>
          <text class="label">错误</text>
        </view>
      </view>

      <view v-if="issues.length" class="issues">
        <text class="issues-title">需要修正的行</text>
        <view v-for="(it, idx) in issues.slice(0, 20)" :key="idx" class="issue">
          <text class="issue-row">第 {{ it.row }} 行</text>
          <text class="issue-msg">{{ it.field }}：{{ it.message }}</text>
        </view>
        <text v-if="issues.length > 20" class="hint">仅显示前 20 条，共 {{ issues.length }} 条</text>
      </view>

      <button class="btn primary" :disabled="preview.batch.error_count > 0 || busy" :loading="busy" @tap="commit">
        确认导入 {{ preview.to_create }} 人
      </button>
    </view>

    <view v-if="lastBatch" class="card done">
      <text class="done-title">已导入 {{ lastBatch.created_rows }} 人</text>
      <text class="hint">跳过 {{ lastBatch.skipped_rows }} 人</text>
      <button class="btn ghost" :loading="busy" @tap="rollback">撤销本次导入</button>
    </view>
  </view>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ApiError, request } from "@/utils/request";

interface ImportIssue {
  row: number;
  field: string;
  message: string;
}

interface ImportBatch {
  id: string;
  status: string;
  total_rows: number;
  created_rows: number;
  skipped_rows: number;
  error_count: number;
  issues: ImportIssue[] | null;
}

interface ImportPreview {
  batch: ImportBatch;
  to_create: number;
  to_skip: number;
}

interface TemplateResponse {
  filename: string;
  content_base64: string;
}

interface FileSystemManager {
  writeFile(options: { filePath: string; data: string; encoding: "base64"; success: () => void; fail: (err: { errMsg: string }) => void }): void;
  readFile(options: { filePath: string; encoding: "base64"; success: (res: { data: string }) => void; fail: (err: { errMsg: string }) => void }): void;
}

const wxApi = (globalThis as unknown as {
  wx?: {
    env?: { USER_DATA_PATH?: string };
    getFileSystemManager?: () => FileSystemManager;
    openDocument?: (options: { filePath: string; showMenu?: boolean; fail?: (err: { errMsg: string }) => void }) => void;
  };
}).wx;

const busy = ref(false);
const preview = ref<ImportPreview | null>(null);
const lastBatch = ref<ImportBatch | null>(null);
const issues = ref<ImportIssue[]>([]);

function toast(title: string): void {
  uni.showToast({ title, icon: "none" });
}

function handleError(err: unknown): void {
  if (err instanceof ApiError && err.status === 401) {
    uni.removeStorageSync("hm_token");
    uni.reLaunch({ url: "/pkg-teacher/pages/login/index" });
    return;
  }
  toast(err instanceof Error ? err.message : "操作失败");
}

async function downloadTemplate(): Promise<void> {
  busy.value = true;
  try {
    const res = await request<TemplateResponse>("/api/v1/imports/students/template", "GET");
    const fs = wxApi?.getFileSystemManager?.();
    const dir = wxApi?.env?.USER_DATA_PATH;
    if (!fs || !dir) {
      toast("请在微信开发者工具/小程序中打开");
      return;
    }
    const filePath = `${dir}/students-template.xlsx`;
    fs.writeFile({
      filePath,
      data: res.content_base64,
      encoding: "base64",
      success: () => {
        wxApi?.openDocument?.({ filePath, showMenu: true, fail: () => toast("已保存，请在文件管理器中打开") });
      },
      fail: (err) => toast(err.errMsg || "模板保存失败"),
    });
  } catch (err) {
    handleError(err);
  } finally {
    busy.value = false;
  }
}

function chooseFile(): void {
  uni.chooseMessageFile({
    count: 1,
    type: "file",
    extension: ["xlsx", "xls"],
    success: (res) => {
      const file = res.tempFiles?.[0];
      if (file?.path) {
        void readAndPreview(file.path);
      }
    },
    fail: (err) => toast(err.errMsg || "未选择文件"),
  });
}

async function readAndPreview(filePath: string): Promise<void> {
  const fs = wxApi?.getFileSystemManager?.();
  if (!fs) {
    toast("请在微信小程序中导入");
    return;
  }
  busy.value = true;
  try {
    const base64 = await new Promise<string>((resolve, reject) => {
      fs.readFile({
        filePath,
        encoding: "base64",
        success: (res) => resolve(res.data),
        fail: (err) => reject(new Error(err.errMsg || "读取文件失败")),
      });
    });
    // callContainer 请求体上限 100KiB；base64 会膨胀约 4/3。
    if (base64.length > 80 * 1024) {
      toast("文件超过约 60KB，当前上传通道放不下；请拆分后再导入");
      return;
    }
    const res = await request<ImportPreview>("/api/v1/imports/students/preview", "POST", {
      content_base64: base64,
      mode: "create",
    });
    preview.value = res;
    issues.value = res.batch.issues ?? [];
    lastBatch.value = null;
    toast(`解析 ${res.batch.total_rows} 行`);
  } catch (err) {
    handleError(err);
  } finally {
    busy.value = false;
  }
}

async function commit(): Promise<void> {
  const batch = preview.value?.batch;
  if (!batch) return;
  busy.value = true;
  try {
    const res = await request<{ batch: ImportBatch }>(`/api/v1/imports/${batch.id}/commit`, "POST");
    lastBatch.value = res.batch;
    preview.value = null;
    issues.value = [];
    toast("导入完成");
  } catch (err) {
    handleError(err);
  } finally {
    busy.value = false;
  }
}

async function rollback(): Promise<void> {
  const batch = lastBatch.value;
  if (!batch) return;
  busy.value = true;
  try {
    await request(`/api/v1/imports/${batch.id}/rollback`, "POST");
    lastBatch.value = null;
    toast("已撤销本次导入");
  } catch (err) {
    handleError(err);
  } finally {
    busy.value = false;
  }
}
</script>

<style scoped lang="scss">
.page {
  min-height: 100vh;
  padding: 32rpx 32rpx calc(48rpx + env(safe-area-inset-bottom));
  background: #f3f7f6;
  box-sizing: border-box;
}
.hero {
  padding: 16rpx 8rpx 28rpx;
}
.hero-title {
  display: block;
  font-size: 44rpx;
  font-weight: 700;
  color: #113b36;
}
.hero-sub {
  display: block;
  margin-top: 8rpx;
  font-size: 26rpx;
  color: #6b7d79;
}
.card {
  margin-bottom: 24rpx;
  padding: 32rpx;
  border-radius: 28rpx;
  background: #ffffff;
  box-shadow: 0 10rpx 30rpx rgba(17, 59, 54, 0.06);
}
.btn {
  width: 100%;
  margin: 0 0 20rpx;
  border-radius: 18rpx;
  font-size: 30rpx;
  line-height: 92rpx;
}
.btn::after {
  border: none;
}
.primary {
  background: #0f766e;
  color: #ffffff;
}
.ghost {
  background: #e8f4f1;
  color: #0f5f57;
}
.btn[disabled] {
  opacity: 0.45;
}
.hint {
  display: block;
  font-size: 24rpx;
  line-height: 1.6;
  color: #7b8c88;
}
.summary {
  display: flex;
  justify-content: space-between;
  margin-bottom: 20rpx;
}
.metric {
  flex: 1;
  text-align: center;
}
.num {
  display: block;
  font-size: 46rpx;
  font-weight: 700;
  color: #0f766e;
}
.label {
  font-size: 24rpx;
  color: #7b8c88;
}
.issues {
  margin: 12rpx 0 28rpx;
  padding: 20rpx;
  border-radius: 16rpx;
  background: #fff6ed;
}
.issues-title {
  display: block;
  margin-bottom: 12rpx;
  font-size: 26rpx;
  font-weight: 600;
  color: #9a5b20;
}
.issue {
  margin-bottom: 10rpx;
}
.issue-row {
  margin-right: 10rpx;
  font-size: 24rpx;
  color: #9a5b20;
}
.issue-msg {
  font-size: 24rpx;
  color: #7a4a1a;
}
.done {
  text-align: center;
}
.done-title {
  display: block;
  margin-bottom: 8rpx;
  font-size: 34rpx;
  font-weight: 700;
  color: #0f5f57;
}
</style>
