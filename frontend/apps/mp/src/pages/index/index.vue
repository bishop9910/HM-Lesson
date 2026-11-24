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
const hasGuardian = ref(false);

const sysInfo = uni.getSystemInfoSync();
const statusBarHeight = sysInfo.statusBarHeight ?? 20;

// 顶部留出状态栏 + 微信胶囊按钮的高度，避免内容被胶囊和时间遮住。
const menuButton = (() => {
  try {
    return uni.getMenuButtonBoundingClientRect();
  } catch {
    return null;
  }
})();
const topbarHeight =
  menuButton && menuButton.bottom > 0 ? menuButton.bottom + 8 : statusBarHeight + 44;

const todoCount = computed(() => {
  if (!summary.value) return 0;
  return (
    summary.value.submissions_pending +
    summary.value.bookings_pending +
    summary.value.low_balance_students
  );
});

const greeting = computed(() => {
  if (!session.isTeacher) return "家长您好";
  const name = session.staffName || "老师";
  return summary.value ? `${name}，今天有 ${todoCount.value} 项待办` : name;
});

const cards = computed(() => {
  if (!summary.value) return [];
  return [
    { title: "待批阅", value: summary.value.submissions_pending },
    { title: "待审核", value: summary.value.bookings_pending },
    { title: "课时提醒", value: summary.value.low_balance_students },
  ];
});

async function loadSummary(): Promise<void> {
  if (!session.isTeacher) {
    summary.value = null;
    return;
  }
  loading.value = true;
  try {
    const res = await request<{ summary: Summary }>("/api/v1/dashboard/summary", "GET");
    summary.value = res.summary;
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      session.clear();
      summary.value = null;
    }
  } finally {
    loading.value = false;
  }
}

onShow(() => {
  hasGuardian.value = Boolean(uni.getStorageSync("hm_guardian_token"));
  void loadSummary();
});

function goBind(): void {
  uni.navigateTo({ url: "/pages/bind/index" });
}

function goGuardian(): void {
  if (hasGuardian.value) {
    uni.navigateTo({ url: "/pages/portal/index" });
    return;
  }
  uni.navigateTo({ url: "/pages/guardian-login/index" });
}

function goTeacher(): void {
  if (!session.isTeacher) {
    uni.navigateTo({ url: "/pkg-teacher/pages/login/index" });
    return;
  }
  uni.navigateTo({ url: "/pkg-teacher/pages/workbench/index" });
}
</script>

<template>
  <view class="shell">
    <view
      class="topbar"
      :style="{ paddingTop: `${statusBarHeight}px`, height: `${topbarHeight}px` }"
    >
      <text class="brand">鹤鸣作业本</text>
    </view>

    <view class="page">
      <view class="hero">
        <text class="title">{{ greeting }}</text>
        <text v-if="session.isTeacher && loading" class="hint">正在同步…</text>
      </view>

      <view v-if="cards.length" class="cards">
        <view v-for="card in cards" :key="card.title" class="card">
          <text class="card-title">{{ card.title }}</text>
          <text class="card-value">{{ card.value }}</text>
        </view>
      </view>

      <view v-else-if="!session.isTeacher" class="empty">
        <text class="empty-title">还没有绑定学员</text>
        <text class="empty-hint">绑定后可以查看作业、课时和预约。</text>
      </view>

      <view class="actions">
        <button v-if="!session.isTeacher" class="primary" @click="goGuardian">
          {{ hasGuardian ? "我的孩子" : "家长登录" }}
        </button>
        <button v-if="!session.isTeacher" class="ghost" @click="goBind">绑定学员</button>
        <button class="ghost" @click="goTeacher">
          {{ session.isTeacher ? "进入工作台" : "教师登录" }}
        </button>
      </view>
    </view>
  </view>
</template>

<style lang="scss" scoped>
@import "@/styles/tokens.scss";

.shell {
  min-height: 100vh;
  background: $hm-bg;
}

.topbar {
  display: flex;
  align-items: center;
  box-sizing: border-box;
  padding-right: 220rpx;
  padding-left: $hm-space;
}

.brand {
  font-size: 28rpx;
  font-weight: 600;
  letter-spacing: 2rpx;
  color: $hm-primary-strong;
}

.page {
  padding: 16rpx $hm-space calc(48rpx + env(safe-area-inset-bottom));
  box-sizing: border-box;
}

.hero {
  display: flex;
  flex-direction: column;
  gap: 10rpx;
  padding: 44rpx 32rpx;
  border-radius: 32rpx;
  background: linear-gradient(135deg, #0f766e, #134e4a);
  color: #f0fdfa;
}

.title {
  font-size: 40rpx;
  font-weight: 700;
  line-height: 1.4;
}

.hint {
  font-size: 24rpx;
  opacity: 0.8;
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
  padding: 26rpx 20rpx;
  background: $hm-surface;
  border-radius: $hm-radius;
  border: 1rpx solid $hm-border;
}

.card-title {
  font-size: 22rpx;
  color: $hm-muted;
}

.card-value {
  font-size: 44rpx;
  font-weight: 700;
  color: $hm-primary;
}

.empty {
  display: flex;
  flex-direction: column;
  gap: 8rpx;
  margin-top: 28rpx;
  padding: 32rpx 24rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  border-radius: $hm-radius;
}

.empty-title {
  font-size: 30rpx;
  font-weight: 600;
}

.empty-hint {
  font-size: 24rpx;
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
</style>
