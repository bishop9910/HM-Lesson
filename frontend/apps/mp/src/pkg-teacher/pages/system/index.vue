<script setup lang="ts">
import { computed, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";
import { useSessionStore } from "@/stores/session";

interface AuditEntry {
  id: string;
  actor_type: string;
  actor_name: string;
  action: string;
  target_type: string;
  target_id: string;
  after_summary?: string;
  created_at: string;
}

interface PermissionRow {
  id: string;
  phone: string;
  name: string;
  role: string;
  status: string;
  hidden: boolean;
  can_manage: boolean;
  can_review: boolean;
  can_export: boolean;
  caveats: string[];
}

interface QueryResult {
  columns: string[];
  rows: string[][];
  truncated: boolean;
  max_rows: number;
}

const session = useSessionStore();
const tab = ref<"audit" | "permissions" | "console">("audit");
const loading = ref(false);
const errorText = ref("");

const audits = ref<AuditEntry[]>([]);
const permissions = ref<PermissionRow[]>([]);

const sqlText = ref("");
const querying = ref(false);
const queryError = ref("");
const result = ref<QueryResult | null>(null);

const COLUMN_WIDTH = 220;
const tableWidth = computed(() => `${Math.max(1, result.value?.columns.length ?? 1) * COLUMN_WIDTH}rpx`);

const roleLabels: Record<string, string> = { owner: "负责人", assistant: "助教" };
const statusLabels: Record<string, string> = { active: "生效", disabled: "停用" };

async function loadAudit(): Promise<void> {
  loading.value = true;
  errorText.value = "";
  try {
    const res = await request<{ items: AuditEntry[] }>("/api/v1/system/audit?limit=50", "GET");
    audits.value = res.items;
  } catch (err) {
    errorText.value = err instanceof ApiError ? err.message : "加载失败";
  } finally {
    loading.value = false;
  }
}

async function loadPermissions(): Promise<void> {
  loading.value = true;
  errorText.value = "";
  try {
    const res = await request<{ items: PermissionRow[] }>("/api/v1/system/permissions", "GET");
    permissions.value = res.items;
  } catch (err) {
    errorText.value = err instanceof ApiError ? err.message : "加载失败";
  } finally {
    loading.value = false;
  }
}

function switchTab(next: "audit" | "permissions" | "console"): void {
  tab.value = next;
  if (next === "audit") void loadAudit();
  if (next === "permissions") void loadPermissions();
}

function shortTime(iso: string): string {
  if (!iso) return "";
  return iso.slice(5, 16).replace("T", " ");
}

async function runQuery(): Promise<void> {
  const stmt = sqlText.value.trim();
  if (!stmt) {
    queryError.value = "请输入要执行的查询";
    return;
  }
  querying.value = true;
  queryError.value = "";
  result.value = null;
  try {
    result.value = await request<QueryResult>("/api/v1/system/query", "POST", { sql: stmt });
  } catch (err) {
    queryError.value = err instanceof ApiError ? err.message : "执行失败";
  } finally {
    querying.value = false;
  }
}

function useSample(): void {
  sqlText.value = "SELECT id, phone, role, status, hidden FROM staff WHERE deleted_at IS NULL";
}

function copyRow(line: string[]): void {
  uni.setClipboardData({ data: line.map((cell) => (cell === null ? "NULL" : String(cell))).join("\t") });
}

onShow(() => {
  if (!session.isTeacher) {
    uni.reLaunch({ url: "/pkg-teacher/pages/login/index" });
    return;
  }
  if (!session.isSystem) {
    uni.reLaunch({ url: "/pkg-teacher/pages/workbench/index" });
    return;
  }
  void loadAudit();
});
</script>

<template>
  <view class="page">
    <text class="title">系统维护</text>
    <text class="subtitle">操作记录、权限视图与只读查询；仅内部系统账号可用。</text>

    <view class="tabs">
      <view class="tab" :class="{ active: tab === 'audit' }" @click="switchTab('audit')">操作记录</view>
      <view class="tab" :class="{ active: tab === 'permissions' }" @click="switchTab('permissions')">权限</view>
      <view class="tab" :class="{ active: tab === 'console' }" @click="switchTab('console')">控制台</view>
    </view>

    <view v-if="errorText" class="error">{{ errorText }}</view>

    <template v-if="tab === 'audit'">
      <view v-if="loading" class="hint">正在加载…</view>
      <view v-else-if="!audits.length" class="hint">
        还没有操作记录。删除学员、清理账号等动作会在这里留下流水。
      </view>
      <view v-for="item in audits" :key="item.id" class="card">
        <view class="row">
          <text class="action">{{ item.action }}</text>
          <text class="time">{{ shortTime(item.created_at) }}</text>
        </view>
        <text class="meta">
          {{ item.actor_name || item.actor_type }} · {{ item.target_type }}
          {{ item.target_id ? `#${item.target_id.slice(0, 8)}` : "" }}
        </text>
        <text v-if="item.after_summary" class="detail">{{ item.after_summary }}</text>
      </view>
    </template>

    <template v-else-if="tab === 'permissions'">
      <view v-if="loading" class="hint">正在加载…</view>
      <view v-for="item in permissions" :key="item.id" class="card">
        <view class="row">
          <text class="action">{{ item.name }}</text>
          <text class="badge">{{ roleLabels[item.role] ?? item.role }}</text>
        </view>
        <text class="meta">
          {{ item.phone }} · {{ statusLabels[item.status] ?? item.status }}
          <text v-if="item.hidden"> · 隐藏账号</text>
        </text>
        <view class="flags">
          <text class="flag" :class="{ on: item.can_manage }">成员管理</text>
          <text class="flag" :class="{ on: item.can_review }">批阅</text>
          <text class="flag" :class="{ on: item.can_export }">导出</text>
        </view>
        <text v-for="note in item.caveats" :key="note" class="detail">{{ note }}</text>
      </view>
    </template>

    <template v-else>
      <view class="console">
        <textarea
          v-model="sqlText"
          class="editor"
          placeholder="只读查询：SELECT / SHOW / DESCRIBE / EXPLAIN，单条语句，最多返回 200 行"
          :maxlength="4000"
        />
        <view class="console-actions">
          <button class="ghost" @click="useSample">填入示例</button>
          <button class="primary" :loading="querying" :disabled="querying" @click="runQuery">执行</button>
        </view>
      </view>

      <view v-if="queryError" class="error">{{ queryError }}</view>

      <view v-if="result" class="result">
        <text class="meta">
          {{ result.rows.length }} 行<text v-if="result.truncated">（已截断，最多 {{ result.max_rows }} 行）</text>
        </text>
        <scroll-view class="table-view" scroll-x>
          <view class="table" :style="{ width: tableWidth }">
            <view class="thead">
              <view v-for="col in result.columns" :key="col" class="cell head">{{ col }}</view>
            </view>
            <view v-for="(line, i) in result.rows" :key="i" class="trow" @click="copyRow(line)">
              <view v-for="(value, j) in line" :key="j" class="cell">{{ value }}</view>
            </view>
          </view>
        </scroll-view>
        <text class="hint-line">左右拖动查看全部列，点按任意一行复制整行</text>
      </view>
    </template>
  </view>
</template>

<style lang="scss" scoped>
@import "@/styles/tokens.scss";

.page {
  min-height: 100vh;
  padding: 32rpx $hm-space calc(48rpx + env(safe-area-inset-bottom));
  box-sizing: border-box;
  background: $hm-bg;
}

.title {
  display: block;
  font-size: 38rpx;
  font-weight: 700;
}

.subtitle {
  display: block;
  margin-top: 10rpx;
  font-size: 24rpx;
  color: $hm-muted;
  line-height: 1.5;
}

.tabs {
  display: flex;
  margin-top: 24rpx;
  padding: 6rpx;
  border-radius: 40rpx;
  background: $hm-paper-deep;
}

.tab {
  flex: 1;
  height: 64rpx;
  line-height: 64rpx;
  text-align: center;
  border-radius: 32rpx;
  font-size: 26rpx;
  color: $hm-muted;
}

.tab.active {
  background: $hm-surface;
  color: $hm-primary-strong;
  font-weight: 600;
}

.error {
  margin-top: 20rpx;
  padding: 18rpx 22rpx;
  border-radius: 12rpx;
  background: rgba(220, 38, 38, 0.08);
  color: $hm-danger;
  font-size: 24rpx;
}

.hint {
  margin-top: 28rpx;
  padding: 32rpx;
  border-radius: $hm-radius;
  background: $hm-surface;
  border: 1rpx dashed $hm-border;
  color: $hm-muted;
  font-size: 26rpx;
  line-height: 1.6;
}

.card {
  margin-top: 20rpx;
  padding: 24rpx;
  border-radius: $hm-radius;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

.row {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
}

.action {
  font-size: 28rpx;
  font-weight: 600;
  color: $hm-ink;
}

.time,
.meta {
  font-size: 24rpx;
  color: $hm-muted;
}

.meta {
  display: block;
  margin-top: 8rpx;
}

.detail {
  display: block;
  margin-top: 8rpx;
  font-size: 22rpx;
  color: $hm-muted;
  word-break: break-all;
}

.badge {
  padding: 4rpx 16rpx;
  border-radius: 20rpx;
  background: $hm-paper-deep;
  font-size: 22rpx;
  color: $hm-primary-strong;
}

.flags {
  display: flex;
  flex-wrap: wrap;
  margin-top: 12rpx;
}

.flag {
  margin: 0 12rpx 8rpx 0;
  padding: 6rpx 16rpx;
  border-radius: 20rpx;
  border: 1rpx solid $hm-border;
  font-size: 22rpx;
  color: $hm-muted;
}

.flag.on {
  border-color: $hm-primary;
  color: $hm-primary-strong;
  background: rgba(15, 118, 110, 0.08);
}

.console {
  margin-top: 24rpx;
  padding: 24rpx;
  border-radius: $hm-radius;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

.editor {
  width: 100%;
  height: 260rpx;
  padding: 20rpx;
  box-sizing: border-box;
  border-radius: 12rpx;
  background: $hm-paper;
  font-size: 24rpx;
  font-family: $hm-serif;
}

.console-actions {
  display: flex;
  margin-top: 16rpx;
}

.console-actions > .ghost,
.console-actions > .primary {
  flex: 1;
  height: 76rpx;
  line-height: 76rpx;
  border-radius: 12rpx;
  font-size: 26rpx;
}

.console-actions > .ghost {
  margin-right: 16rpx;
  background: $hm-paper-deep;
  color: $hm-ink;
}

.console-actions > .primary {
  background: $hm-primary;
  color: #fff;
}

.result {
  margin-top: 24rpx;
}

.table-view {
  margin-top: 12rpx;
  width: 100%;
  border-radius: 12rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

/* scroll-view 的横向滚动取决于直接子节点的真实宽度：
   这里按「列数 × 每列宽度」算出确定宽度，避免 flex 内容宽度算不出来导致拖不动 */
.table {
  display: block;
  box-sizing: border-box;
  padding: 12rpx 0;
}

.thead,
.trow {
  display: flex;
  align-items: flex-start;
}

.thead {
  border-bottom: 1rpx solid $hm-border;
}

.cell {
  flex: 0 0 220rpx;
  width: 220rpx;
  padding: 10rpx 16rpx;
  box-sizing: border-box;
  font-size: 22rpx;
  line-height: 1.45;
  color: $hm-ink;
  text-align: left;
  word-break: break-all;
  white-space: normal;
  overflow: hidden;
}

.cell.head {
  font-weight: 600;
  color: $hm-primary-strong;
}

.hint-line {
  display: block;
  margin-top: 10rpx;
  font-size: 22rpx;
  color: $hm-muted;
}
</style>
