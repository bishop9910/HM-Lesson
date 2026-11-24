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
}

const home = ref<Home | null>(null);
const loading = ref(false);
const errorText = ref("");

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

onShow(() => {
  void load();
});

function logout(): void {
  uni.removeStorageSync("hm_guardian_token");
  uni.removeStorageSync("hm_guardian_phone");
  uni.reLaunch({ url: "/pages/index/index" });
}

function goHome(): void {
  uni.reLaunch({ url: "/pages/index/index" });
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

    <view v-if="errorText" class="error">{{ errorText }}</view>
    <view v-else-if="loading && !home" class="hint">正在加载…</view>

    <template v-if="home">
      <view class="stats">
        <view class="stat">
          <text class="stat-value">{{ home.students.length }}</text>
          <text class="stat-label">在读学员</text>
        </view>
        <view class="stat">
          <text class="stat-value">{{ home.pending_tasks }}</text>
          <text class="stat-label">待交作业</text>
        </view>
        <view class="stat">
          <text class="stat-value">{{ home.balance_hours }}</text>
          <text class="stat-label">剩余课时</text>
        </view>
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

.stats {
  display: flex;
  gap: 14rpx;
  margin-top: 24rpx;
}

.stat {
  flex: 1;
  padding: 22rpx 18rpx;
  border-radius: $hm-radius;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
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
