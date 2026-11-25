<script setup lang="ts">
import { computed, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";

import { ApiError, request } from "@/utils/request";

interface NoticeItem {
  id: string;
  title: string;
  body: string;
  need_receipt: boolean;
  status: string;
  sent_at: string;
  created_at: string;
  my_receipt_status: string;
  my_confirmed_at: string;
}

const loading = ref(true);
const error = ref("");
const items = ref<NoticeItem[]>([]);
const confirming = ref("");
const expanded = ref("");

const pending = computed(() => items.value.filter((item) => item.need_receipt && item.my_receipt_status !== "confirmed"));

function shortTime(iso: string): string {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return `${date.getMonth() + 1}月${date.getDate()}日 ${String(date.getHours()).padStart(2, "0")}:${String(
    date.getMinutes(),
  ).padStart(2, "0")}`;
}

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    const res = await request<{ items: NoticeItem[] }>("/api/v1/portal/notices", "GET", undefined, "guardian");
    items.value = res.items;
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      uni.removeStorageSync("hm_guardian_token");
      uni.reLaunch({ url: "/pages/guardian-login/index" });
      return;
    }
    error.value = err instanceof ApiError ? err.message : "读取通知失败";
  } finally {
    loading.value = false;
  }
}

function toggle(id: string): void {
  expanded.value = expanded.value === id ? "" : id;
}

async function confirm(item: NoticeItem): Promise<void> {
  if (confirming.value) return;
  confirming.value = item.id;
  try {
    await request<{ ok: boolean }>(`/api/v1/portal/notices/${item.id}/confirm`, "POST", {}, "guardian");
    uni.showToast({ title: "已确认", icon: "success" });
    await load();
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "确认失败", icon: "none" });
  } finally {
    confirming.value = "";
  }
}

onShow(() => {
  void load();
});
</script>

<template>
  <view class="page">
    <view class="head">
      <text class="title">消息中心</text>
      <text class="sub">老师发布的通知都在这里，需回执的请点「已确认」</text>
    </view>

    <view v-if="loading" class="hint">加载中…</view>
    <view v-else-if="error" class="hint" @tap="load">{{ error }}，点我重试</view>
    <view v-else-if="items.length === 0" class="empty">还没有收到通知。</view>

    <view v-if="pending.length > 0" class="banner">有 {{ pending.length }} 条通知待你确认</view>

    <view v-for="item in items" :key="item.id" class="card" @tap="toggle(item.id)">
      <view class="card-top">
        <text class="name">{{ item.title }}</text>
        <text v-if="item.need_receipt" class="badge" :class="{ ok: item.my_receipt_status === 'confirmed' }">
          {{ item.my_receipt_status === "confirmed" ? "已确认" : "待确认" }}
        </text>
      </view>
      <text class="time">{{ shortTime(item.sent_at || item.created_at) }}</text>
      <text class="body" :class="{ clamp: expanded !== item.id }">{{ item.body }}</text>
      <view v-if="item.need_receipt && item.my_receipt_status !== 'confirmed'" class="actions">
        <view class="btn primary" :class="{ disabled: confirming === item.id }" @tap.stop="confirm(item)">
          {{ confirming === item.id ? "提交中…" : "我已确认" }}
        </view>
      </view>
      <text v-if="item.my_confirmed_at" class="confirmed-at">已于 {{ shortTime(item.my_confirmed_at) }} 确认</text>
    </view>
  </view>
</template>

<style lang="scss" scoped>
@import "@/styles/tokens.scss";

.page {
  min-height: 100vh;
  padding: 24rpx 24rpx 80rpx;
  background: $hm-bg;
}

.head {
  padding: 8rpx 4rpx 20rpx;
}

.title {
  display: block;
  font-size: 44rpx;
  font-weight: 600;
  color: $hm-ink;
}

.sub {
  display: block;
  margin-top: 6rpx;
  font-size: 24rpx;
  color: $hm-muted;
}

.banner {
  margin-bottom: 16rpx;
  padding: 18rpx 22rpx;
  border-radius: 14rpx;
  background: rgba(217, 119, 6, 0.12);
  border: 1rpx solid rgba(217, 119, 6, 0.4);
  color: $hm-accent;
  font-size: 26rpx;
}

.card {
  padding: 22rpx 24rpx;
  margin-bottom: 16rpx;
  border-radius: $hm-radius;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

.card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.name {
  flex: 1;
  font-size: 32rpx;
  font-weight: 600;
  color: $hm-ink;
}

.badge {
  padding: 2rpx 14rpx;
  border-radius: 999rpx;
  background: rgba(217, 119, 6, 0.12);
  color: $hm-accent;
  font-size: 22rpx;
}

.badge.ok {
  background: rgba(15, 118, 110, 0.12);
  color: $hm-primary;
}

.time {
  display: block;
  margin-top: 6rpx;
  font-size: 22rpx;
  color: $hm-muted;
}

.body {
  display: block;
  margin-top: 10rpx;
  font-size: 26rpx;
  color: $hm-ink;
  line-height: 1.6;
}

.body.clamp {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.actions {
  margin-top: 16rpx;
}

.btn {
  height: 72rpx;
  line-height: 72rpx;
  border-radius: 14rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  color: $hm-ink;
  font-size: 28rpx;
  text-align: center;
}

.btn.primary {
  background: $hm-primary;
  border-color: $hm-primary;
  color: #fff;
}

.btn.disabled {
  opacity: 0.5;
}

.confirmed-at {
  display: block;
  margin-top: 12rpx;
  font-size: 22rpx;
  color: $hm-muted;
}

.hint,
.empty {
  padding: 60rpx 0;
  text-align: center;
  color: $hm-muted;
  font-size: 26rpx;
}
</style>
