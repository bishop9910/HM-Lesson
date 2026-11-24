<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";

interface Student {
  id: string;
  name: string;
  grade: string;
  school: string;
  group_code: string;
  status: string;
}
interface Task {
  id: string;
  title: string;
  status: string;
  due_at?: string;
}
interface Lesson {
  id: string;
  kind: string;
  delta_hours: number;
  balance_after: number;
  note: string;
  created_at: string;
}
interface Detail {
  student: Student;
  tasks: Task[];
  balance_hours: number;
  expires_at?: string;
  lessons: Lesson[];
}

const id = ref("");
const detail = ref<Detail | null>(null);
const loading = ref(false);
const errorText = ref("");

const statusLabels: Record<string, string> = {
  pending: "待提交",
  submitted: "已提交",
  reviewed: "已批阅",
  returned: "已打回",
};
const kindLabels: Record<string, string> = {
  purchase: "购买",
  consume: "消课",
  adjust: "调整",
  refund: "退课",
};
const pendingCount = computed(
  () => detail.value?.tasks.filter((t) => t.status === "pending" || t.status === "returned").length ?? 0,
);

async function load(): Promise<void> {
  if (!id.value) return;
  loading.value = true;
  errorText.value = "";
  try {
    detail.value = await request<Detail>(`/api/v1/portal/students/${id.value}`, "GET", undefined, "guardian");
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

onLoad((options) => {
  id.value = options?.id ?? "";
  void load();
});

function goBack(): void {
  uni.navigateBack();
}
</script>

<template>
  <view class="page">
    <view v-if="errorText" class="error">{{ errorText }}</view>
    <view v-else-if="loading && !detail" class="hint">正在加载…</view>

    <template v-if="detail">
      <view class="hero">
        <text class="name">{{ detail.student.name }}</text>
        <text class="meta">{{ detail.student.grade }} · {{ detail.student.school || "未填学校" }}</text>
        <view class="balance">
          <text class="balance-value">{{ detail.balance_hours }}</text>
          <text class="balance-label">剩余课时</text>
        </view>
        <text v-if="detail.expires_at" class="meta">有效期至 {{ detail.expires_at }}</text>
      </view>

      <view class="section">
        <view class="section-head">
          <text class="section-title">作业</text>
          <text class="section-sub">待完成 {{ pendingCount }}</text>
        </view>
        <view v-if="!detail.tasks.length" class="hint">暂时没有作业。</view>
        <view v-for="task in detail.tasks" :key="task.id" class="row">
          <view class="row-main">
            <text class="row-title">{{ task.title }}</text>
            <text class="row-sub">{{ task.due_at ? `截止 ${task.due_at.slice(0, 10)}` : "未设截止" }}</text>
          </view>
          <text class="tag">{{ statusLabels[task.status] || task.status }}</text>
        </view>
      </view>

      <view class="section">
        <text class="section-title">课时记录</text>
        <view v-if="!detail.lessons.length" class="hint">还没有课时流水。</view>
        <view v-for="lesson in detail.lessons" :key="lesson.id" class="row">
          <view class="row-main">
            <text class="row-title">{{ kindLabels[lesson.kind] || lesson.kind }}{{ lesson.note ? ` · ${lesson.note}` : "" }}</text>
            <text class="row-sub">{{ lesson.created_at.slice(0, 10) }} · 余额 {{ lesson.balance_after }}</text>
          </view>
          <text class="delta" :class="{ plus: lesson.delta_hours > 0 }">
            {{ lesson.delta_hours > 0 ? "+" : "" }}{{ lesson.delta_hours }}
          </text>
        </view>
      </view>

      <button class="ghost" @click="goBack">返回</button>
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

.hero {
  padding: 30rpx;
  border-radius: 28rpx;
  background: linear-gradient(135deg, #0f766e, #134e4a);
  color: #f0fdfa;
}

.name {
  display: block;
  font-size: 42rpx;
  font-weight: 700;
}

.meta {
  display: block;
  margin-top: 8rpx;
  font-size: 24rpx;
  opacity: 0.85;
}

.balance {
  display: flex;
  align-items: baseline;
  gap: 12rpx;
  margin-top: 20rpx;
}

.balance-value {
  font-size: 56rpx;
  font-weight: 700;
}

.balance-label {
  font-size: 24rpx;
  opacity: 0.8;
}

.section {
  margin-top: 24rpx;
  padding: 24rpx;
  border-radius: $hm-radius;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

.section-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}

.section-title {
  font-size: 30rpx;
  font-weight: 600;
}

.section-sub {
  font-size: 24rpx;
  color: $hm-muted;
}

.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16rpx;
  margin-top: 18rpx;
  padding-top: 16rpx;
  border-top: 1rpx dashed $hm-border;
}

.row-main {
  flex: 1;
  min-width: 0;
}

.row-title {
  display: block;
  font-size: 28rpx;
  line-height: 1.4;
}

.row-sub {
  display: block;
  margin-top: 4rpx;
  font-size: 22rpx;
  color: $hm-muted;
}

.tag {
  flex: none;
  padding: 4rpx 14rpx;
  border-radius: 18rpx;
  font-size: 22rpx;
  color: $hm-primary-strong;
  background: #ccfbf1;
}

.delta {
  flex: none;
  font-size: 30rpx;
  font-weight: 600;
  color: #b45309;
}

.delta.plus {
  color: $hm-primary;
}

.hint,
.error {
  margin-top: 18rpx;
  font-size: 24rpx;
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
}
</style>
