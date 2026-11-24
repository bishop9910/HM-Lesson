<script setup lang="ts">
import { computed, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";

interface Task {
  id: string;
  title: string;
  description: string;
  type: string;
  status: string;
  due_at?: string;
  assigned_count: number;
  submitted_count: number;
  pending_review: number;
  overdue_count: number;
  submit_rate: number;
}

interface Student {
  id: string;
  name: string;
  grade: string;
  group_code: string;
}

interface TaskTemplate {
  id: string;
  title: string;
  description: string;
  type: string;
  due_offset_hours: number;
}

const typeLabels: Record<string, string> = {
  homework: "常规作业",
  correction: "试卷订正",
  trial: "试听任务",
  checkin: "打卡",
};
const statusLabels: Record<string, string> = {
  draft: "草稿",
  published: "进行中",
  closed: "已结束",
};
const filters = [
  { value: "", label: "全部" },
  { value: "draft", label: "草稿" },
  { value: "published", label: "进行中" },
  { value: "closed", label: "已结束" },
];

const tasks = ref<Task[]>([]);
const total = ref(0);
const status = ref("");
const q = ref("");
const loading = ref(false);
const error = ref("");
const showCreate = ref(false);
const showTemplates = ref(false);
const templates = ref<TaskTemplate[]>([]);
const students = ref<Student[]>([]);
const selected = ref<string[]>([]);
const saving = ref(false);

const title = ref("");
const description = ref("");
const type = ref("homework");
const dueDate = ref(defaultDate());
const dueTime = ref("20:00");

function defaultDate(): string {
  const d = new Date();
  d.setDate(d.getDate() + 1);
  const m = `${d.getMonth() + 1}`.padStart(2, "0");
  const day = `${d.getDate()}`.padStart(2, "0");
  return `${d.getFullYear()}-${m}-${day}`;
}

function fmt(iso?: string): string {
  if (!iso) return "未设置";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return `${d.getMonth() + 1}月${d.getDate()}日 ${`${d.getHours()}`.padStart(2, "0")}:${`${d.getMinutes()}`.padStart(2, "0")}`;
}

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    const query = [`page=1`];
    if (status.value) query.push(`status=${status.value}`);
    if (q.value.trim()) query.push(`q=${encodeURIComponent(q.value.trim())}`);
    const res = await request<{ items: Task[]; total: number }>(`/api/v1/tasks?${query.join("&")}`, "GET");
    tasks.value = res.items;
    total.value = res.total;
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : "加载失败";
  } finally {
    loading.value = false;
  }
}

async function loadStudents(): Promise<void> {
  try {
    const res = await request<{ items: Student[] }>("/api/v1/students?page=1&status=active", "GET");
    students.value = res.items;
  } catch {
    students.value = [];
  }
}

function openCreate(): void {
  showCreate.value = true;
  if (students.value.length === 0) void loadStudents();
}

async function openTemplates(): Promise<void> {
  try {
    const res = await request<{ items: TaskTemplate[] }>("/api/v1/task-templates", "GET");
    templates.value = res.items;
    showTemplates.value = true;
  } catch {
    uni.showToast({ title: "模板加载失败", icon: "none" });
  }
}

function applyTemplate(t: TaskTemplate): void {
  title.value = t.title;
  description.value = t.description;
  type.value = t.type;
  const d = new Date();
  d.setHours(d.getHours() + t.due_offset_hours);
  dueDate.value = `${d.getFullYear()}-${`${d.getMonth() + 1}`.padStart(2, "0")}-${`${d.getDate()}`.padStart(2, "0")}`;
  dueTime.value = `${`${d.getHours()}`.padStart(2, "0")}:${`${d.getMinutes()}`.padStart(2, "0")}`;
  showTemplates.value = false;
  showCreate.value = true;
  if (students.value.length === 0) void loadStudents();
}

async function saveTemplate(): Promise<void> {
  if (!title.value.trim()) {
    uni.showToast({ title: "先填写标题", icon: "none" });
    return;
  }
  const due = new Date(`${dueDate.value}T${dueTime.value}:00+08:00`).getTime();
  const offset = Math.max(1, Math.min(720, Math.round((due - Date.now()) / 3600000)));
  try {
    await request("/api/v1/task-templates", "POST", {
      title: title.value.trim(),
      description: description.value.trim(),
      type: type.value,
      due_offset_hours: offset,
    });
    uni.showToast({ title: "模板已保存", icon: "success" });
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : "保存失败", icon: "none" });
  }
}

function toggleStudent(id: string): void {
  const i = selected.value.indexOf(id);
  if (i >= 0) selected.value.splice(i, 1);
  else selected.value.push(id);
}

function selectAll(): void {
  selected.value = selected.value.length === students.value.length ? [] : students.value.map((s) => s.id);
}

const canSave = computed(() => title.value.trim().length > 0 && selected.value.length > 0);

async function save(publish: boolean): Promise<void> {
  if (!canSave.value) {
    uni.showToast({ title: "请填写标题并选择学员", icon: "none" });
    return;
  }
  saving.value = true;
  try {
    const iso = new Date(`${dueDate.value}T${dueTime.value}:00+08:00`).toISOString();
    const created = await request<{ task: Task }>("/api/v1/tasks", "POST", {
      title: title.value.trim(),
      description: description.value.trim(),
      type: type.value,
      due_at: iso,
    });
    if (publish) {
      await request(`/api/v1/tasks/${created.task.id}/publish`, "POST", { student_ids: selected.value });
    }
    uni.showToast({ title: publish ? "已发布" : "已保存草稿", icon: "success" });
    showCreate.value = false;
    title.value = "";
    description.value = "";
    selected.value = [];
    await load();
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : "保存失败", icon: "none" });
  } finally {
    saving.value = false;
  }
}

function openTask(id: string): void {
  uni.navigateTo({ url: `/pkg-teacher/pages/task/index?id=${id}` });
}

onShow(load);
</script>

<template>
  <view class="page">
    <view class="toolbar">
      <input v-model="q" class="search" placeholder="搜索任务标题" confirm-type="search" @confirm="load" />
      <button class="ghost" size="mini" @tap="openTemplates">模板</button>
      <button class="primary" size="mini" @tap="openCreate">新建任务</button>
    </view>

    <scroll-view scroll-x class="filters">
      <view v-for="f in filters" :key="f.value" class="chip" :class="{ on: status === f.value }" @tap="status = f.value; load()">
        {{ f.label }}
      </view>
    </scroll-view>

    <view v-if="loading" class="hint">加载中…</view>
    <view v-else-if="error" class="hint error" @tap="load">{{ error }}，点此重试</view>
    <view v-else-if="tasks.length === 0" class="empty">
      <text class="empty-title">还没有任务</text>
      <text class="empty-sub">新建后选择学员下发，一人一条提交记录。</text>
    </view>

    <view v-for="t in tasks" :key="t.id" class="card" @tap="openTask(t.id)">
      <view class="row">
        <text class="title">{{ t.title }}</text>
        <text class="badge" :class="t.status">{{ statusLabels[t.status] }}</text>
      </view>
      <view class="meta">
        <text>{{ typeLabels[t.type] || t.type }}</text>
        <text>截止 {{ fmt(t.due_at) }}</text>
      </view>
      <view class="stats">
        <view class="stat"><text class="num">{{ t.assigned_count }}</text><text class="lbl">下发</text></view>
        <view class="stat"><text class="num">{{ t.submitted_count }}</text><text class="lbl">已交</text></view>
        <view class="stat"><text class="num">{{ t.pending_review }}</text><text class="lbl">待批</text></view>
        <view class="stat danger"><text class="num">{{ t.overdue_count }}</text><text class="lbl">逾期</text></view>
      </view>
      <view class="bar"><view class="bar-in" :style="{ width: `${Math.round(t.submit_rate * 100)}%` }" /></view>
    </view>

    <view v-if="showCreate" class="mask" @tap.self="showCreate = false">
      <view class="sheet" @click.stop>
        <view class="sheet-head">
          <text class="sheet-title">新建任务</text>
          <text class="close" @tap="showCreate = false">关闭</text>
        </view>
        <input v-model="title" class="field" placeholder="任务标题" maxlength="128" />
        <textarea v-model="description" class="field area" placeholder="任务说明（选填）" maxlength="2000" />
        <view class="field-row">
          <picker :range="Object.values(typeLabels)" @change="type = Object.keys(typeLabels)[Number($event.detail.value)]">
            <view class="picker">{{ typeLabels[type] }}</view>
          </picker>
          <picker mode="date" :value="dueDate" @change="dueDate = $event.detail.value">
            <view class="picker">{{ dueDate }}</view>
          </picker>
          <picker mode="time" :value="dueTime" @change="dueTime = $event.detail.value">
            <view class="picker">{{ dueTime }}</view>
          </picker>
        </view>

        <view class="select-head">
          <text>下发学员（{{ selected.length }}）</text>
          <text class="link" @tap="selectAll">全选/取消</text>
        </view>
        <scroll-view scroll-y class="student-list">
          <view v-for="s in students" :key="s.id" class="student" :class="{ on: selected.includes(s.id) }" @tap="toggleStudent(s.id)">
            <text>{{ s.name }}</text>
            <text class="grade">{{ s.grade }} · {{ s.group_code }}</text>
          </view>
          <view v-if="students.length === 0" class="hint">暂无在读学员，请先添加学员。</view>
        </scroll-view>

        <view class="actions">
          <button class="ghost" :disabled="saving" @tap="saveTemplate">存为模板</button>
          <button class="ghost" :disabled="saving" @tap="save(false)">保存草稿</button>
          <button class="primary" :disabled="saving" @tap="save(true)">保存并发布</button>
        </view>
      </view>
    </view>

    <view v-if="showTemplates" class="mask" @tap.self="showTemplates = false">
      <view class="sheet" @click.stop>
        <view class="sheet-head">
          <text class="sheet-title">任务模板</text>
          <text class="close" @tap="showTemplates = false">关闭</text>
        </view>
        <view v-if="templates.length === 0" class="hint">还没有模板。新建任务时点“存为模板”。</view>
        <view v-for="t in templates" :key="t.id" class="template" @tap="applyTemplate(t)">
          <text class="title">{{ t.title }}</text>
          <text class="meta">{{ typeLabels[t.type] }} · 默认 {{ t.due_offset_hours }} 小时后截止</text>
        </view>
      </view>
    </view>
  </view>
</template>

<style lang="scss">
@import "@/styles/tokens.scss";
.page { min-height: 100vh; background: $hm-bg; padding: 24rpx 24rpx 48rpx; }
.toolbar { display: flex; gap: 16rpx; align-items: center; margin-bottom: 20rpx; }
.search { flex: 1; height: 72rpx; padding: 0 24rpx; background: rgba(255, 255, 255, 0.55); border: 1rpx solid $hm-rule; border-radius: 10rpx; font-size: 26rpx; }
.primary { background: $hm-primary-strong; color: #fff; border-radius: 10rpx; font-size: 26rpx; }
.ghost { background: transparent; color: $hm-primary-strong; border: 1rpx solid $hm-rule; border-radius: 10rpx; font-size: 26rpx; }
.filters { white-space: nowrap; margin-bottom: 20rpx; }
.chip { display: inline-block; padding: 10rpx 24rpx; margin-right: 12rpx; border-radius: 999rpx; background: transparent; color: $hm-muted; border: 1rpx solid $hm-rule; font-size: 24rpx; }
.chip.on { background: $hm-primary-strong; color: #fff; border-color: $hm-primary-strong; }
.card { background: rgba(255, 255, 255, 0.38); border: 1rpx solid $hm-rule; border-radius: 10rpx; padding: 26rpx; margin-bottom: 20rpx; }
.row { display: flex; justify-content: space-between; gap: 16rpx; align-items: center; }
.title { font-size: 30rpx; font-weight: 700; color: $hm-text; flex: 1; }
.badge { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 999rpx; background: rgba(34, 48, 42, 0.07); color: $hm-muted; }
.badge.published { background: #d9ece4; color: #115e59; }
.badge.closed { background: rgba(34, 48, 42, 0.07); color: $hm-muted; }
.badge.draft { background: #fef3c7; color: #92400e; }
.meta { display: flex; gap: 20rpx; color: $hm-muted; font-size: 23rpx; margin-top: 10rpx; }
.template { padding: 22rpx 0; border-bottom: 1rpx solid $hm-border; }
.template .title { display: block; }
.template .meta { display: block; }
.stats { display: flex; gap: 30rpx; margin-top: 22rpx; }
.stat { display: flex; flex-direction: column; }
.num { font-size: 34rpx; font-weight: 700; color: $hm-text; }
.lbl { font-size: 21rpx; color: $hm-muted; }
.stat.danger .num { color: $hm-danger; }
.bar { height: 8rpx; background: #e2e8f0; border-radius: 999rpx; margin-top: 20rpx; overflow: hidden; }
.bar-in { height: 100%; background: $hm-primary; }
.hint { color: $hm-muted; text-align: center; padding: 40rpx 0; font-size: 26rpx; }
.hint.error { color: $hm-danger; }
.empty { background: rgba(255, 255, 255, 0.38); border: 1rpx solid $hm-rule; border-radius: 10rpx; padding: 60rpx 30rpx; text-align: center; }
.empty-title { display: block; font-size: 30rpx; color: $hm-text; font-weight: 600; }
.empty-sub { display: block; font-size: 24rpx; color: $hm-muted; margin-top: 10rpx; }
.mask { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.45); display: flex; align-items: flex-end; z-index: 20; }
.sheet { width: 100%; max-height: 88vh; overflow-y: auto; -webkit-overflow-scrolling: touch; background: $hm-paper; border-radius: 28rpx 28rpx 0 0; padding: 30rpx 24rpx calc(30rpx + env(safe-area-inset-bottom)); box-sizing: border-box; }
.sheet-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 24rpx; }
.sheet-title { font-family: $hm-serif; font-size: 34rpx; font-weight: 700; color: $hm-ink; }
.close, .link { color: $hm-primary-strong; font-size: 25rpx; }
.field { display: block; width: 100%; height: 88rpx; padding: 0 24rpx; line-height: 88rpx; box-sizing: border-box; border: 1rpx solid $hm-rule; background: rgba(255, 255, 255, 0.55); border-radius: 10rpx; font-size: 28rpx; color: $hm-ink; margin-bottom: 18rpx; }
.area { height: 180rpx; padding: 20rpx 24rpx; line-height: 1.6; }
.field-row { display: flex; gap: 12rpx; margin-bottom: 22rpx; }
.field-row picker { flex: 1; }
.picker { display: block; background: rgba(255, 255, 255, 0.55); border: 1rpx solid $hm-rule; border-radius: 10rpx; padding: 22rpx 12rpx; text-align: center; font-size: 25rpx; color: $hm-ink; }
.select-head { display: flex; justify-content: space-between; font-size: 25rpx; color: $hm-ink; margin: 6rpx 0 14rpx; }
.student-list { max-height: 320rpx; }
.student { display: flex; justify-content: space-between; align-items: center; padding: 20rpx; border-radius: 10rpx; background: rgba(255, 255, 255, 0.55); border: 1rpx solid $hm-rule; margin-bottom: 10rpx; font-size: 26rpx; }
.student.on { background: #d9ece4; border-color: $hm-primary; color: #115e59; }
.grade { font-size: 22rpx; color: rgba(34, 48, 42, 0.55); }
.actions { display: flex; gap: 16rpx; margin-top: 24rpx; }
.actions button { flex: 1; }
.actions .primary { background: $hm-primary-strong; color: #f7f3e8; }
.actions .ghost { background: transparent; color: $hm-primary-strong; border: 1rpx solid $hm-rule; }
</style>
