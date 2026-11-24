<script setup lang="ts">
import { ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";

interface Assignment {
  id: string;
  task_title: string;
  task_type: string;
  status: string;
  due_at?: string;
  reviewed_at?: string;
  score?: number;
  comment: string;
}

const items = ref<Assignment[]>([]);
const loading = ref(true);
const error = ref("");

function fmt(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  return `${d.getFullYear()}-${`${d.getMonth() + 1}`.padStart(2, "0")}-${`${d.getDate()}`.padStart(2, "0")}`;
}

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    const res = await request<{ items: Assignment[] }>("/api/v1/portal/archive", "GET", undefined, "guardian");
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
    <view class="head">
      <text class="title">作业档案</text>
      <text class="sub">{{ items.length }} 次已批阅</text>
    </view>
    <view v-if="loading" class="hint">加载中…</view>
    <view v-else-if="error" class="hint error" @tap="load">{{ error }}，点此重试</view>
    <view v-else-if="items.length === 0" class="empty">
      <text class="empty-title">还没有批阅记录</text>
      <text class="empty-sub">老师批阅后的作业会归档在这里。</text>
    </view>
    <view v-for="a in items" :key="a.id" class="card" @tap="open(a)">
      <view class="row">
        <text class="name">{{ a.task_title }}</text>
        <text v-if="a.score !== undefined" class="score">{{ a.score }} 分</text>
      </view>
      <text class="meta">批阅 {{ fmt(a.reviewed_at) }} · 截止 {{ fmt(a.due_at) }}</text>
      <text v-if="a.comment" class="comment">{{ a.comment }}</text>
    </view>
  </view>
</template>

<style lang="scss">
@import "@/styles/tokens.scss";
.page { min-height: 100vh; background: $hm-bg; padding: 24rpx 24rpx 60rpx; }
.head { display: flex; justify-content: space-between; align-items: baseline; margin-bottom: 20rpx; }
.title { font-size: 34rpx; font-weight: 700; }
.sub { color: $hm-muted; font-size: 23rpx; }
.card { background: #fff; border-radius: $hm-radius; padding: 24rpx; margin-bottom: 18rpx; }
.row { display: flex; justify-content: space-between; gap: 16rpx; }
.name { font-size: 28rpx; font-weight: 600; flex: 1; }
.score { color: $hm-accent; font-weight: 700; font-size: 26rpx; }
.meta { display: block; color: $hm-muted; font-size: 22rpx; margin-top: 8rpx; }
.comment { display: block; color: #334155; font-size: 24rpx; line-height: 1.6; margin-top: 10rpx; max-height: 96rpx; overflow: hidden; }
.hint { color: $hm-muted; text-align: center; padding: 40rpx 0; }
.hint.error { color: $hm-danger; }
.empty { background: #fff; border-radius: $hm-radius; padding: 60rpx 30rpx; text-align: center; }
.empty-title { display: block; font-size: 28rpx; font-weight: 600; }
.empty-sub { display: block; color: $hm-muted; font-size: 24rpx; margin-top: 10rpx; }
</style>
