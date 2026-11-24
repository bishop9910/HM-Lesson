<script setup lang="ts">
import { onLoad } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";
import { useSessionStore } from "@/stores/session";

const session = useSessionStore();

onLoad(async () => {
  if (!session.isTeacher) {
    uni.reLaunch({ url: "/pkg-teacher/pages/login/index" });
    return;
  }
  try {
    await request("/api/v1/auth/me", "GET");
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      session.clear();
      uni.reLaunch({ url: "/pkg-teacher/pages/login/index" });
    }
  }
});

const todos = [
  { title: "待批阅作业", value: "2", hint: "含 1 份逾期提交" },
  { title: "待审核预约", value: "1", hint: "今天 18:00 前处理" },
  { title: "课时不足提醒", value: "1", hint: "余额 ≤ 2 课时" },
];

function goStaff(): void {
  uni.navigateTo({ url: "/pkg-teacher/pages/staff/index" });
}

function logout(): void {
  session.clear();
  uni.reLaunch({ url: "/pages/index/index" });
}
</script>

<template>
  <view class="page">
    <view class="header">
      <text class="title">教师工作台</text>
      <text class="who">{{ session.staffName || "老师" }} · {{ session.role }}</text>
    </view>

    <view v-for="item in todos" :key="item.title" class="row">
      <view class="row-main">
        <text class="row-title">{{ item.title }}</text>
        <text class="row-hint">{{ item.hint }}</text>
      </view>
      <text class="row-value">{{ item.value }}</text>
    </view>

    <button v-if="session.isOwner" class="primary" @click="goStaff">成员管理</button>
    <button class="ghost" @click="logout">退出登录</button>
    <text class="footnote">M1 骨架：批阅/审核页面按 M2 接入；账号与成员管理已接通。</text>
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

.row-main {
  display: flex;
  flex-direction: column;
  gap: 8rpx;
}

.row-title {
  font-size: 30rpx;
  font-weight: 600;
}

.row-hint {
  font-size: 22rpx;
  color: $hm-muted;
}

.row-value {
  font-size: 40rpx;
  font-weight: 700;
  color: $hm-accent;
}

.primary,
.ghost {
  margin-top: 40rpx;
  height: 88rpx;
  line-height: 88rpx;
  border-radius: 44rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  color: $hm-text;
}

.primary {
  margin-top: 20rpx;
  background: $hm-primary;
  border-color: $hm-primary;
  color: #ffffff;
}

.footnote {
  display: block;
  margin-top: 24rpx;
  font-size: 22rpx;
  color: $hm-muted;
}
</style>
