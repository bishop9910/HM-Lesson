<script setup lang="ts">
import { computed, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";

interface Booking {
  id: string;
  slot_id: string;
  student_id: string;
  student_name: string;
  grade: string;
  guardian_name: string;
  status: string;
  request_note: string;
  review_note: string;
  attendance: string;
  start_at: string;
  end_at: string;
  submitted_at: string;
}

const TABS = [
  { key: "pending", label: "待审核" },
  { key: "approved", label: "已通过" },
  { key: "completed", label: "已完成" },
  { key: "", label: "全部" },
];

const activeTab = ref("pending");
const items = ref<Booking[]>([]);
const loading = ref(true);
const error = ref("");
const acting = ref("");

const statusLabel: Record<string, string> = {
  pending: "待审核",
  approved: "已通过",
  rejected: "已驳回",
  completed: "已完成",
  cancelled: "已撤回",
  no_show: "未到课",
};

const emptyText = computed(() =>
  activeTab.value === "pending" ? "没有待审核的预约" : "没有符合条件的预约",
);

function fmt(iso: string): string {
  const d = new Date(iso);
  return `${d.getMonth() + 1}月${d.getDate()}日 ${`${d.getHours()}`.padStart(2, "0")}:${`${d.getMinutes()}`.padStart(2, "0")}`;
}

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    const query = activeTab.value ? `?status=${activeTab.value}` : "";
    const res = await request<{ items: Booking[] }>(`/api/v1/bookings${query}`, "GET");
    items.value = res.items;
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : "加载失败";
  } finally {
    loading.value = false;
  }
}

function switchTab(key: string): void {
  if (activeTab.value === key) return;
  activeTab.value = key;
  void load();
}

async function approve(booking: Booking): Promise<void> {
  acting.value = booking.id;
  try {
    await request(`/api/v1/bookings/${booking.id}/approve`, "POST", {});
    uni.showToast({ title: "已通过", icon: "success" });
    await load();
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : "操作失败", icon: "none" });
  } finally {
    acting.value = "";
  }
}

async function reject(booking: Booking): Promise<void> {
  const res = await uni.showModal({
    title: "驳回预约",
    editable: true,
    placeholderText: "填写驳回理由（家长可见）",
    confirmText: "驳回",
  });
  if (!res.confirm) return;
  const reason = (res.content ?? "").trim();
  if (!reason) {
    uni.showToast({ title: "请填写理由", icon: "none" });
    return;
  }
  acting.value = booking.id;
  try {
    await request(`/api/v1/bookings/${booking.id}/reject`, "POST", { reason });
    uni.showToast({ title: "已驳回", icon: "success" });
    await load();
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : "操作失败", icon: "none" });
  } finally {
    acting.value = "";
  }
}

async function complete(booking: Booking): Promise<void> {
  const res = await uni.showModal({
    title: "标记已上课",
    content: `确认 ${booking.student_name} 已到课并扣减课时？`,
    confirmText: "已上课",
  });
  if (!res.confirm) return;
  acting.value = booking.id;
  try {
    const out = await request<{ hours: number }>(`/api/v1/bookings/${booking.id}/complete`, "POST", {});
    uni.showToast({ title: `已消课 ${out.hours}`, icon: "success" });
    await load();
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : "操作失败", icon: "none" });
  } finally {
    acting.value = "";
  }
}

async function noShow(booking: Booking): Promise<void> {
  const res = await uni.showModal({
    title: "标记未到课",
    editable: true,
    placeholderText: "备注（可选）",
    content: "",
    confirmText: "未到课",
  });
  if (!res.confirm) return;
  acting.value = booking.id;
  try {
    await request(`/api/v1/bookings/${booking.id}/no-show`, "POST", { note: (res.content ?? "").trim() });
    uni.showToast({ title: "已标记未到课", icon: "success" });
    await load();
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : "操作失败", icon: "none" });
  } finally {
    acting.value = "";
  }
}

onShow(load);
</script>

<template>
  <view class="page">
    <view class="tabs">
      <text
        v-for="tab in TABS"
        :key="tab.key"
        class="tab"
        :class="{ active: activeTab === tab.key }"
        @tap="switchTab(tab.key)"
      >
        {{ tab.label }}
      </text>
    </view>

    <view v-if="loading" class="hint">加载中…</view>
    <view v-else-if="error" class="hint error" @tap="load">{{ error }}，点此重试</view>
    <view v-else-if="items.length === 0" class="hint">{{ emptyText }}</view>

    <view v-for="item in items" :key="item.id" class="card">
      <view class="row">
        <text class="name">{{ item.student_name }}</text>
        <text class="badge" :class="item.status">{{ statusLabel[item.status] ?? item.status }}</text>
      </view>
      <text class="when">{{ fmt(item.start_at) }} – {{ fmt(item.end_at).slice(-5) }}</text>
      <text v-if="item.grade" class="meta">{{ item.grade }} · 家长 {{ item.guardian_name }}</text>
      <text v-if="item.request_note" class="note">诉求：{{ item.request_note }}</text>
      <text v-if="item.review_note" class="note">意见：{{ item.review_note }}</text>

      <view v-if="item.status === 'pending'" class="actions">
        <button class="btn" :loading="acting === item.id" @tap="approve(item)">通过</button>
        <button class="btn ghost danger" :loading="acting === item.id" @tap="reject(item)">驳回</button>
      </view>
      <view v-else-if="item.status === 'approved'" class="actions">
        <button class="btn" :loading="acting === item.id" @tap="complete(item)">已上课·消课</button>
        <button class="btn ghost" :loading="acting === item.id" @tap="noShow(item)">未到课</button>
      </view>
    </view>
  </view>
</template>

<style lang="scss">
@import "@/styles/tokens.scss";
.page { min-height: 100vh; background: $hm-bg; padding: 24rpx 24rpx 60rpx; }
.tabs { display: flex; margin-bottom: 20rpx; }
.tab { font-size: 25rpx; color: $hm-muted; padding: 10rpx 24rpx; border-radius: 999rpx; border: 1rpx solid $hm-rule; margin-right: 12rpx; }
.tab.active { background: $hm-primary; border-color: $hm-primary; color: #fff; }
.card { background: $hm-surface; border: 1rpx solid $hm-rule; border-radius: $hm-radius; padding: 24rpx; margin-bottom: 18rpx; }
.row { display: flex; justify-content: space-between; align-items: center; }
.name { font-size: 29rpx; font-weight: 600; }
.badge { font-size: 22rpx; color: $hm-muted; }
.badge.pending { color: $hm-accent; }
.badge.approved { color: $hm-primary; }
.badge.no_show { color: $hm-danger; }
.when { display: block; font-size: 25rpx; margin-top: 6rpx; }
.meta { display: block; color: $hm-muted; font-size: 23rpx; margin-top: 4rpx; }
.note { display: block; color: #475569; font-size: 24rpx; margin-top: 10rpx; line-height: 1.6; }
.actions { display: flex; margin-top: 20rpx; }
.btn { background: $hm-primary; color: #fff; font-size: 25rpx; border-radius: 12rpx; margin-right: 16rpx; padding: 0 28rpx; }
.btn.ghost { background: transparent; color: $hm-primary; border: 1rpx solid $hm-primary; }
.btn.ghost.danger { color: $hm-danger; border-color: $hm-danger; }
.hint { color: $hm-muted; text-align: center; padding: 40rpx 0; font-size: 25rpx; }
.hint.error { color: $hm-danger; }
</style>
