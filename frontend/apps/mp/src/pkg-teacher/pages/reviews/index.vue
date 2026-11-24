<script setup lang="ts">
import { ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";

interface Assignment {
  id: string;
  task_id: string;
  task_title: string;
  task_type: string;
  student_name: string;
  status: string;
  submitted_at?: string;
  due_at?: string;
  submission?: { text: string; file_ids: string[]; is_late: boolean };
}

const items = ref<Assignment[]>([]);
const loading = ref(true);
const error = ref("");

function fmt(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  return `${d.getMonth() + 1}月${d.getDate()}日 ${`${d.getHours()}`.padStart(2, "0")}:${`${d.getMinutes()}`.padStart(2, "0")}`;
}

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    const res = await request<{ items: Assignment[] }>("/api/v1/reviews", "GET");
    items.value = res.items;
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : "加载失败";
  } finally {
    loading.value = false;
  }
}

function open(a: Assignment): void {
  uni.navigateTo({ url: `/pkg-teacher/pages/task/index?id=${a.task_id}` });
}

onShow(load);
</script>

<template>
  <view class="page">
    <view class="head">
      <text class="title">待批阅</text>
      <text class="sub">{{ items.length }} 份</text>
    </view>
    <view v-if="loading" class="hint">加载中…</view>
    <view v-else-if="error" class="hint error" @tap="load">{{ error }}，点此重试</view>
    <view v-else-if="items.length === 0" class="empty">
      <text class="empty-title">没有待批阅的作业</text>
      <text class="empty-sub">家长提交后会出现在这里。</text>
    </view>
    <view v-for="a in items" :key="a.id" class="card" @tap="open(a)">
      <view class="row">
        <text class="name">{{ a.student_name }}</text>
        <text v-if="a.submission?.is_late" class="late">逾期</text>
      </view>
      <text class="task">{{ a.task_title }}</text>
      <text v-if="a.submission?.text" class="text">{{ a.submission.text }}</text>
      <view class="foot">
        <text>提交 {{ fmt(a.submitted_at) }}</text>
        <text class="go">去批阅 ›</text>
      </view>
    </view>
  </view>
</template>

<style lang="scss">
@import "@/styles/tokens.scss";
.page { min-height: 100vh; background: $hm-bg; padding: 24rpx 24rpx 60rpx; }
.head { display: flex; justify-content: space-between; align-items: baseline; margin-bottom: 20rpx; }
.title { font-size: 34rpx; font-weight: 700; }
.sub { color: $hm-muted; font-size: 24rpx; }
.card { background: rgba(255, 255, 255, 0.38); border: 1rpx solid $hm-rule; border-radius: 10rpx; padding: 24rpx; margin-bottom: 18rpx; }
.row { display: flex; justify-content: space-between; }
.name { font-size: 29rpx; font-weight: 600; }
.late { color: $hm-danger; font-size: 22rpx; }
.task { display: block; color: $hm-primary; font-size: 25rpx; margin-top: 6rpx; }
.text { display: block; color: #475569; font-size: 24rpx; margin-top: 12rpx; line-height: 1.6; max-height: 96rpx; overflow: hidden; }
.foot { display: flex; justify-content: space-between; color: $hm-muted; font-size: 22rpx; margin-top: 16rpx; }
.go { color: $hm-primary; }
.hint { color: $hm-muted; text-align: center; padding: 40rpx 0; }
.hint.error { color: $hm-danger; }
.empty { background: rgba(255, 255, 255, 0.38); border: 1rpx solid $hm-rule; border-radius: 10rpx; padding: 60rpx 30rpx; text-align: center; }
.empty-title { display: block; font-size: 29rpx; font-weight: 600; }
.empty-sub { display: block; color: $hm-muted; font-size: 24rpx; margin-top: 10rpx; }
</style>
