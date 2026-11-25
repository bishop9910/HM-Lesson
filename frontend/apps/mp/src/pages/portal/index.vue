<script setup lang="ts">
import { computed, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";

interface Student {
  id: string;
  name: string;
  grade: string;
  school: string;
  group_code: string;
  status: string;
}
interface Home {
  guardian: { name: string; phone: string };
  students: Student[];
  pending_tasks: number;
  balance_hours: number;
  pending_receipts: number;
  upcoming_bookings: number;
  next_booking_at: string;
  materials_count: number;
  latest_material_name: string;
}
interface NotificationItem {
  id: string;
  kind: string;
  title: string;
  body: string;
  ref_type: string;
  ref_id: string;
}

interface SubscriptionItem {
  template_key: string;
  template_id: string;
  remaining: number;
  configured: boolean;
}
interface WxSubscribeResult {
  errMsg?: string;
  [templateID: string]: unknown;
}
interface WxSubscribeApi {
  requestSubscribeMessage(options: {
    tmplIds: string[];
    success?: (res: WxSubscribeResult) => void;
    fail?: (err: { errMsg?: string }) => void;
  }): void;
}

const home = ref<Home | null>(null);
const loading = ref(false);
const errorText = ref("");
const subscriptions = ref<SubscriptionItem[]>([]);
const subscribing = ref(false);

const groupLabels: Record<string, string> = {
  class_active: "班课在读",
  one_on_one: "一对一在读",
  intent: "待转化意向",
  visitor: "咨询访客",
};
const phone = computed(() => home.value?.guardian.phone ?? "");

async function load(): Promise<void> {
  const token = uni.getStorageSync("hm_guardian_token") as string;
  if (!token) {
    uni.reLaunch({ url: "/pages/guardian-login/index" });
    return;
  }
  loading.value = true;
  errorText.value = "";
  try {
    home.value = await request<Home>("/api/v1/portal/home", "GET", undefined, "guardian");
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      uni.removeStorageSync("hm_guardian_token");
      uni.reLaunch({ url: "/pages/guardian-login/index" });
      return;
    }
    errorText.value = err instanceof ApiError ? err.message : "加载失败";
  } finally {
    loading.value = false;
  }
}

async function loadNotifications(): Promise<void> {
  if (!uni.getStorageSync("hm_guardian_token")) return;
  try {
    const res = await request<{ items: NotificationItem[] }>("/api/v1/portal/notifications", "GET", undefined, "guardian");
    const items = res.items || [];
    if (items.length === 0) return;
    const content = items.slice(0, 3).map((n) => n.title).join("\n");
    uni.showModal({
      title: `你有 ${items.length} 条新消息`,
      content,
      confirmText: "查看作业",
      cancelText: "稍后",
      success: async (r) => {
        try {
          await request("/api/v1/portal/notifications/read-all", "POST", {}, "guardian");
        } catch {
          // 标记失败不阻断查看
        }
        if (r.confirm) goTasks();
      },
    });
  } catch {
    // 通知属于增强能力，失败不打断门户
  }
}

onShow(() => {
  void load();
  void loadNotifications();
  void loadSubscriptions();
});

const reminderRemaining = computed(() =>
  subscriptions.value.reduce((sum, item) => sum + (item.configured ? item.remaining : 0), 0),
);
const reminderAvailable = computed(() => subscriptions.value.some((item) => item.configured && item.template_id));

async function loadSubscriptions(): Promise<void> {
  if (!uni.getStorageSync("hm_guardian_token")) return;
  try {
    const res = await request<{ items: SubscriptionItem[]; enabled: boolean }>(
      "/api/v1/portal/subscriptions",
      "GET",
      undefined,
      "guardian",
    );
    subscriptions.value = res.items || [];
  } catch {
    // 订阅能力属于增强项，失败不打断门户
  }
}

function normalizeState(value: unknown): string {
  return typeof value === "string" ? value : "unknown";
}

function enableReminders(): void {
  if (subscribing.value) return;
  const configured = subscriptions.value.filter((item) => item.configured && item.template_id);
  if (configured.length === 0) {
    uni.showToast({ title: "老师还没有配置提醒模板", icon: "none" });
    return;
  }
  const wxApi = (globalThis as unknown as { wx?: WxSubscribeApi }).wx;
  if (!wxApi || typeof wxApi.requestSubscribeMessage !== "function") {
    uni.showToast({ title: "请在小程序里开启提醒", icon: "none" });
    return;
  }
  subscribing.value = true;
  wxApi.requestSubscribeMessage({
    tmplIds: configured.map((item) => item.template_id),
    success: (res) => {
      void submitGrants(configured, res);
    },
    fail: (err) => {
      subscribing.value = false;
      const cancelled = (err?.errMsg || "").includes("cancel");
      uni.showToast({ title: cancelled ? "已取消" : "开启失败，请稍后再试", icon: "none" });
    },
  });
}

async function submitGrants(configured: SubscriptionItem[], res: WxSubscribeResult): Promise<void> {
  const grants = configured
    .map((item) => ({ template_key: item.template_key, state: normalizeState(res[item.template_id]) }))
    .filter((grant) => grant.state !== "unknown");
  if (grants.length === 0) {
    subscribing.value = false;
    uni.showToast({ title: "未开启提醒", icon: "none" });
    return;
  }
  try {
    const out = await request<{ items: SubscriptionItem[] }>(
      "/api/v1/portal/subscriptions",
      "POST",
      { grants },
      "guardian",
    );
    if (out.items && out.items.length > 0) subscriptions.value = out.items;
    const accepted = grants.filter((grant) => grant.state === "accept").length;
    uni.showToast({ title: accepted > 0 ? `已开启 ${accepted} 项提醒` : "未开启提醒", icon: "none" });
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "开启失败", icon: "none" });
  } finally {
    subscribing.value = false;
  }
}

function logout(): void {
  uni.removeStorageSync("hm_guardian_token");
  uni.removeStorageSync("hm_guardian_phone");
  uni.reLaunch({ url: "/pages/home/index" });
}

function goHome(): void {
  uni.reLaunch({ url: "/pages/home/index" });
}

function openStudent(id: string): void {
  uni.navigateTo({ url: `/pages/portal/student?id=${encodeURIComponent(id)}` });
}

function goTasks(): void {
  uni.navigateTo({ url: "/pages/portal/tasks/index" });
}

function goArchive(): void {
  uni.navigateTo({ url: "/pages/portal/archive" });
}

function goBooking(): void {
  uni.navigateTo({ url: "/pages/portal/booking" });
}

function goMaterials(): void {
  uni.navigateTo({ url: "/pages/portal/materials" });
}

function goPassword(): void {
  uni.navigateTo({ url: "/pages/portal/password" });
}

function goNotices(): void {
  uni.navigateTo({ url: "/pages/portal/notices" });
}
</script>

<template>
  <view class="page">
    <view class="top">
      <view>
        <text class="hello">家长您好</text>
        <text class="phone">{{ phone }}</text>
      </view>
      <text class="link" @click="logout">退出</text>
    </view>

    <view class="account">
      <text class="account-text">账号 {{ phone }}</text>
      <text class="account-link" @click="goPassword">修改密码</text>
    </view>

    <view v-if="errorText" class="error">{{ errorText }}</view>
    <view v-else-if="loading && !home" class="hint">正在加载…</view>

    <template v-if="home">
      <view class="stats">
        <view class="stat" @click="goTasks">
          <text class="stat-value">{{ home.pending_tasks }}</text>
          <text class="stat-label">待交作业</text>
        </view>
        <view class="stat" @click="goNotices">
          <text class="stat-value" :class="{ warn: home.pending_receipts > 0 }">{{ home.pending_receipts }}</text>
          <text class="stat-label">待确认回执</text>
        </view>
        <view class="stat">
          <text class="stat-value" :class="{ warn: home.balance_hours <= 2 }">{{ home.balance_hours }}</text>
          <text class="stat-label">剩余课时</text>
        </view>
        <view class="stat" @click="goBooking">
          <text class="stat-value">{{ home.upcoming_bookings }}</text>
          <text class="stat-label">待上预约</text>
        </view>
        <view class="stat" @click="goMaterials">
          <text class="stat-value">{{ home.materials_count }}</text>
          <text class="stat-label">可看资料</text>
        </view>
      </view>

      <view v-if="home.next_booking_at" class="next-booking">
        <text class="next-label">下次上课</text>
        <text class="next-time">{{ home.next_booking_at.slice(5, 16).replace("T", " ") }}</text>
      </view>

      <view v-if="reminderAvailable" class="reminder">
        <view class="reminder-text">
          <text class="reminder-title">作业提醒</text>
          <text class="reminder-hint">{{
            reminderRemaining > 0
              ? `已开启 ${reminderRemaining} 次微信提醒`
              : "开启后作业发布、批改会收到微信服务通知"
          }}</text>
        </view>
        <button class="reminder-btn" :disabled="subscribing" :loading="subscribing" @click="enableReminders">
          {{ reminderRemaining > 0 ? "再开一次" : "开启提醒" }}
        </button>
      </view>

      <view v-if="!home.students.length" class="empty">
        <text class="empty-title">还没有关联学员</text>
        <text class="empty-hint">请让老师通过你的绑定申请。</text>
      </view>

      <view v-for="student in home.students" :key="student.id" class="card" @click="openStudent(student.id)">
        <view class="card-top">
          <text class="name">{{ student.name }}</text>
          <text class="badge">{{ groupLabels[student.group_code] || student.group_code }}</text>
        </view>
        <text class="meta">{{ student.grade }} · {{ student.school || "未填学校" }}</text>
      </view>

      <view class="quick">
        <button class="ghost" @click="goTasks">我的任务</button>
        <button class="ghost" @click="goBooking">预约上课</button>
      </view>
      <view class="quick">
        <button class="ghost" @click="goMaterials">学习资料</button>
        <button class="ghost" @click="goNotices">消息中心</button>
        <button class="ghost" @click="goArchive">作业档案</button>
      </view>

      <button class="ghost" @click="goHome">回到首页</button>
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

.top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}

.hello {
  display: block;
  font-size: 40rpx;
  font-weight: 700;
}

.phone {
  display: block;
  margin-top: 6rpx;
  font-size: 24rpx;
  color: $hm-muted;
}

.link {
  font-size: 26rpx;
  color: $hm-muted;
}

.quick {
  display: flex;
  gap: 16rpx;
  margin: 20rpx 0;
}

.quick button {
  flex: 1;
}

.account {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 16rpx;
  padding: 16rpx 24rpx;
  border-radius: $hm-radius;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

.account-text {
  font-size: 24rpx;
  color: $hm-muted;
}

.account-link {
  font-size: 24rpx;
  color: $hm-primary-strong;
}

.stats {
  display: flex;
  flex-wrap: wrap;
  gap: 14rpx;
  margin-top: 24rpx;
}

.stat {
  flex: 1 1 28%;
  min-width: 180rpx;
  padding: 22rpx 18rpx;
  border-radius: $hm-radius;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

.stat-value.warn {
  color: $hm-accent;
}

.next-booking {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 14rpx;
  padding: 16rpx 22rpx;
  border-radius: 14rpx;
  background: rgba(15, 118, 110, 0.08);
  border: 1rpx solid rgba(15, 118, 110, 0.24);
}

.next-label {
  font-size: 24rpx;
  color: $hm-muted;
}

.next-time {
  font-size: 26rpx;
  color: $hm-primary-strong;
}

.stat-value {
  display: block;
  font-size: 42rpx;
  font-weight: 700;
  color: $hm-primary;
}

.stat-label {
  display: block;
  margin-top: 4rpx;
  font-size: 22rpx;
  color: $hm-muted;
}

.card {
  margin-top: 18rpx;
  padding: 24rpx;
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
  font-size: 32rpx;
  font-weight: 600;
}

.badge {
  padding: 4rpx 14rpx;
  border-radius: 20rpx;
  font-size: 22rpx;
  color: $hm-primary-strong;
  background: #ccfbf1;
}

.meta {
  display: block;
  margin-top: 8rpx;
  font-size: 24rpx;
  color: $hm-muted;
}

.reminder {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 24rpx;
  padding: 24rpx;
  border-radius: $hm-radius;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

.reminder-text {
  flex: 1;
  margin-right: 20rpx;
}

.reminder-title {
  display: block;
  font-size: 30rpx;
  font-weight: 600;
}

.reminder-hint {
  display: block;
  margin-top: 6rpx;
  font-size: 22rpx;
  color: $hm-muted;
}

.reminder-btn {
  min-width: 180rpx;
  height: 64rpx;
  padding: 0 24rpx;
  line-height: 64rpx;
  border-radius: 32rpx;
  font-size: 24rpx;
  color: #ffffff;
  background: $hm-primary;
}

.empty {
  margin-top: 24rpx;
  padding: 28rpx;
  border-radius: $hm-radius;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

.empty-title {
  display: block;
  font-size: 30rpx;
  font-weight: 600;
}

.empty-hint {
  display: block;
  margin-top: 8rpx;
  font-size: 24rpx;
  color: $hm-muted;
}

.hint,
.error {
  margin-top: 24rpx;
  font-size: 26rpx;
  color: $hm-muted;
}

.error {
  color: #b91c1c;
}

.ghost {
  height: 88rpx;
  margin-top: 28rpx;
  line-height: 88rpx;
  border-radius: 44rpx;
  font-size: 28rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  color: $hm-muted;
}
</style>
