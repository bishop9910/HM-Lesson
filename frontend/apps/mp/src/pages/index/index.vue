<script setup lang="ts">
import { computed } from "vue";
import { useSessionStore } from "@/stores/session";

const session = useSessionStore();

const greeting = computed(() => {
  if (session.isTeacher) return `${session.staffName || "老师"}，今天有 3 项待办`;
  return "家长您好，这里是孩子的作业与课时";
});

function goBind(): void {
  uni.navigateTo({ url: "/pages/bind/index" });
}

function goTeacher(): void {
  if (!session.isTeacher) {
    uni.showToast({ title: "请先登录教师账号", icon: "none" });
    return;
  }
  uni.navigateTo({ url: "/pkg-teacher/pages/workbench/index" });
}

const cards = [
  { title: "待提交作业", value: "1", hint: "截止 11-25 20:00" },
  { title: "本周课时", value: "2", hint: "剩余 12.5 课时" },
  { title: "待确认通知", value: "0", hint: "暂无新通知" },
];
</script>

<template>
  <view class="page">
    <view class="hero">
      <text class="eyebrow">鹤鸣作业本</text>
      <text class="title">{{ greeting }}</text>
      <text class="subtitle">作业收集 · 预约 · 课时台账，一个工具就够了</text>
    </view>

    <view class="cards">
      <view v-for="card in cards" :key="card.title" class="card">
        <text class="card-title">{{ card.title }}</text>
        <text class="card-value">{{ card.value }}</text>
        <text class="card-hint">{{ card.hint }}</text>
      </view>
    </view>

    <view class="actions">
      <button class="primary" @click="goBind">绑定学员</button>
      <button class="ghost" @click="goTeacher">教师工作台</button>
    </view>

    <text class="footnote">当前为 M1 骨架页；数据接口接入后替换为真实数据。</text>
  </view>
</template>

<style lang="scss" scoped>
@import "@/styles/tokens.scss";

.page {
  min-height: 100vh;
  padding: 48rpx $hm-space 64rpx;
  box-sizing: border-box;
}

.hero {
  display: flex;
  flex-direction: column;
  gap: 12rpx;
  padding: 40rpx 32rpx;
  border-radius: 32rpx;
  background: linear-gradient(135deg, #0f766e, #134e4a);
  color: #f0fdfa;
}

.eyebrow {
  font-size: 24rpx;
  letter-spacing: 4rpx;
  opacity: 0.8;
}

.title {
  font-size: 44rpx;
  font-weight: 700;
  line-height: 1.35;
}

.subtitle {
  font-size: 26rpx;
  opacity: 0.85;
}

.cards {
  display: flex;
  gap: 16rpx;
  margin-top: 28rpx;
}

.card {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8rpx;
  padding: 24rpx 20rpx;
  background: $hm-surface;
  border-radius: $hm-radius;
  border: 1rpx solid $hm-border;
}

.card-title {
  font-size: 22rpx;
  color: $hm-muted;
}

.card-value {
  font-size: 40rpx;
  font-weight: 700;
  color: $hm-primary;
}

.card-hint {
  font-size: 20rpx;
  color: $hm-muted;
}

.actions {
  display: flex;
  flex-direction: column;
  gap: 20rpx;
  margin-top: 40rpx;
}

.primary,
.ghost {
  height: 92rpx;
  line-height: 92rpx;
  border-radius: 46rpx;
  font-size: 30rpx;
}

.primary {
  background: $hm-primary;
  color: #ffffff;
}

.ghost {
  background: $hm-surface;
  color: $hm-primary-strong;
  border: 1rpx solid $hm-primary;
}

.footnote {
  display: block;
  margin-top: 32rpx;
  font-size: 22rpx;
  color: $hm-muted;
  text-align: center;
}
</style>
