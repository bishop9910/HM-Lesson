<script setup lang="ts">
import { computed, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";
import { useSessionStore } from "@/stores/session";

interface Summary {
  students_total: number;
  submissions_pending: number;
  bookings_pending: number;
  low_balance_students: number;
  staff_total: number;
}

const session = useSessionStore();
const summary = ref<Summary | null>(null);
const loading = ref(false);
const errorText = ref("");

const todos = computed(() => {
  if (!summary.value) return [];
  return [
    { title: "待批阅作业", value: summary.value.submissions_pending },
    { title: "待审核预约", value: summary.value.bookings_pending },
    { title: "课时不足提醒", value: summary.value.low_balance_students },
  ];
});

async function load(): Promise<void> {
  if (!session.isTeacher) {
    uni.reLaunch({ url: "/pkg-teacher/pages/login/index" });
    return;
  }
  loading.value = true;
  errorText.value = "";
  try {
    await request("/api/v1/auth/me", "GET");
    const res = await request<{ summary: Summary }>("/api/v1/dashboard/summary", "GET");
    summary.value = res.summary;
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      session.clear();
      uni.reLaunch({ url: "/pkg-teacher/pages/login/index" });
      return;
    }
    if (err instanceof ApiError) {
      errorText.value = err.message;
    } else {
      errorText.value = "加载失败，请稍后重试";
    }
  } finally {
    loading.value = false;
  }
}

onShow(() => {
  void load();
});

function goStaff(): void {
  uni.navigateTo({ url: "/pkg-teacher/pages/staff/index" });
}

function goStudents(): void {
  uni.navigateTo({ url: "/pkg-teacher/pages/students/index" });
}

function goGuardians(): void {
  uni.navigateTo({ url: "/pkg-teacher/pages/guardians/index" });
}

function goImport(): void {
  uni.navigateTo({ url: "/pkg-teacher/pages/import/index" });
}

function logout(): void {
  session.clear();
  uni.reLaunch({ url: "/pages/index/index" });
}
</script>

<template>
  <view class="page">
    <view class="header">
      <text class="title">工作台</text>
      <text class="who">{{ session.staffName || "老师" }} · {{ session.role }}</text>
    </view>

    <view v-if="errorText" class="error">{{ errorText }}</view>
    <view v-else-if="loading" class="hint">正在同步…</view>

    <view v-for="item in todos" :key="item.title" class="row">
      <text class="row-title">{{ item.title }}</text>
      <text class="row-value">{{ item.value }}</text>
    </view>

    <view class="tools">
      <button class="primary" @click="goStudents">学员档案</button>
      <button v-if="session.isOwner" class="ghost" @click="goGuardians">绑定审批</button>
      <button v-if="session.isOwner" class="ghost" @click="goImport">导入学员</button>
      <button v-if="session.isOwner" class="ghost" @click="goStaff">成员管理</button>
      <button class="ghost" @click="logout">退出登录</button>
    </view>
  </view>
</template>

<style lang="scss" scoped>
@import "@/styles/tokens.scss";

.page {
  min-height: 100vh;
  padding: 48rpx $hm-space;
  box-sizing: border-box;
  background: $hm-bg;
}

.header {
  display: flex;
  flex-direction: column;
  gap: 8rpx;
}

.title {
  font-size: 40rpx;
  font-weight: 700;
}

.who {
  font-size: 24rpx;
  color: $hm-muted;
}

.hint,
.error {
  display: block;
  margin-top: 24rpx;
  font-size: 26rpx;
  color: $hm-muted;
}

.error {
  color: #b91c1c;
}

.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 20rpx;
  padding: 28rpx 24rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  border-radius: $hm-radius;
}

.row-title {
  font-size: 30rpx;
  font-weight: 600;
}

.row-value {
  font-size: 40rpx;
  font-weight: 700;
  color: $hm-accent;
}

.tools {
  display: flex;
  flex-direction: column;
  gap: 20rpx;
  margin-top: 40rpx;
}

.primary,
.ghost {
  height: 88rpx;
  line-height: 88rpx;
  border-radius: 44rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  color: $hm-text;
}

.primary {
  background: $hm-primary;
  border-color: $hm-primary;
  color: #ffffff;
}
</style>
