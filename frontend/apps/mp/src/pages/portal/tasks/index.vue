<script setup lang="ts">
import { computed, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";

interface Assignment {
  id: string;
  task_id: string;
  task_title: string;
  task_type: string;
  task_description?: string;
  status: string;
  due_at?: string;
  submitted_at?: string;
  score?: number;
  comment: string;
  return_reason: string;
}

const items = ref<Assignment[]>([]);
const loading = ref(true);
const error = ref("");
const tab = ref<"todo" | "done">("todo");

const statusLabels: Record<string, string> = {
  pending: "待提交",
  submitted: "已提交",
  reviewed: "已批阅",
  returned: "已打回",
};

function fmt(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  return `${d.getMonth() + 1}月${d.getDate()}日 ${`${d.getHours()}`.padStart(2, "0")}:${`${d.getMinutes()}`.padStart(2, "0")}`;
}

function overdue(a: Assignment): boolean {
  return a.status === "pending" && Boolean(a.due_at) && new Date(a.due_at!).getTime() < Date.now();
}

const visible = computed(() =>
  items.value.filter((a) => (tab.value === "todo" ? a.status === "pending" || a.status === "returned" : a.status === "submitted" || a.status === "reviewed")),
);

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    const res = await request<{ items: Assignment[] }>("/api/v1/portal/tasks", "GET", undefined, "guardian");
    items.value = res.items;
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      uni.reLaunch({ url: "/pages/guardian-login/index" });
      return;
    }
    error.value = e instanceof ApiError ? e.message : "加载失败";
  } finally {
    loading.value = false;
  }
}

function open(a: Assignment): void {
  uni.navigateTo({ url: `/pages/portal/tasks/detail?id=${a.id}` });
}

onShow(load);
</script>

<template>
  <view class="page">
    <view class="tabs">
      <view class="tab" :class="{ on: tab === 'todo' }" @tap="tab = 'todo'">待完成</view>
      <view class="tab" :class="{ on: tab === 'done' }" @tap="tab = 'done'">已提交</view>
    </view>

    <view v-if="loading" class="hint">加载中…</view>
    <view v-else-if="error" class="hint error" @tap="load">{{ error }}，点此重试</view>
    <view v-else-if="visible.length === 0" class="empty">
      <text class="empty-title">{{ tab === "todo" ? "没有待完成的任务" : "还没有提交记录" }}</text>
      <text class="empty-sub">老师下发后会出现在这里。</text>
    </view>

    <view v-for="a in visible" :key="a.id" class="card" @tap="open(a)">
      <view class="row">
        <text class="title">{{ a.task_title }}</text>
        <text class="badge" :class="a.status">{{ statusLabels[a.status] }}</text>
      </view>
      <view class="meta">
        <text>截止 {{ fmt(a.due_at) }}</text>
        <text v-if="overdue(a)" class="late">已逾期</text>
        <text v-if="a.score !== undefined">评分 {{ a.score }}</text>
      </view>
      <text v-if="a.return_reason" class="reason">打回：{{ a.return_reason }}</text>
    </view>
  </view>
</template>

<style lang="scss">
@import "@/styles/tokens.scss";
.page { min-height: 100vh; background: $hm-bg; padding: 24rpx 24rpx 60rpx; }
.tabs { display: flex; background: #fff; border-radius: 16rpx; padding: 8rpx; margin-bottom: 22rpx; }
.tab { flex: 1; text-align: center; padding: 16rpx; font-size: 26rpx; color: $hm-muted; border-radius: 12rpx; }
.tab.on { background: $hm-primary; color: #fff; }
.card { background: #fff; border-radius: $hm-radius; padding: 24rpx; margin-bottom: 18rpx; }
.row { display: flex; justify-content: space-between; gap: 14rpx; }
.title { font-size: 29rpx; font-weight: 600; color: $hm-text; flex: 1; }
.badge { font-size: 22rpx; padding: 4rpx 12rpx; border-radius: 999rpx; background: #f1f5f9; color: $hm-muted; }
.badge.pending { background: #fef3c7; color: #92400e; }
.badge.reviewed { background: #ccfbf1; color: #115e59; }
.badge.returned { background: #fee2e2; color: #991b1b; }
.badge.submitted { background: #dbeafe; color: #1e40af; }
.meta { display: flex; gap: 18rpx; color: $hm-muted; font-size: 23rpx; margin-top: 12rpx; }
.late { color: $hm-danger; }
.reason { display: block; margin-top: 10rpx; color: #991b1b; font-size: 23rpx; }
.hint { color: $hm-muted; text-align: center; padding: 40rpx 0; }
.hint.error { color: $hm-danger; }
.empty { background: #fff; border-radius: $hm-radius; padding: 60rpx 30rpx; text-align: center; }
.empty-title { display: block; font-size: 28rpx; font-weight: 600; }
.empty-sub { display: block; color: $hm-muted; font-size: 24rpx; margin-top: 10rpx; }
</style>
