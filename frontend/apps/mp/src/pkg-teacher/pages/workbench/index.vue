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

type ActionKey =
  | "tasks"
  | "reviews"
  | "students"
  | "guardians"
  | "import"
  | "staff";

interface ActionItem {
  key: ActionKey;
  title: string;
  desc: string;
  icon: string;
  primary?: boolean;
  ownerOnly?: boolean;
  badge?: number;
}

const session = useSessionStore();
const summary = ref<Summary | null>(null);
const loading = ref(false);
const errorText = ref("");

const roleLabel = computed(() =>
  session.role === "owner" ? "负责人" : session.role === "assistant" ? "助教" : "老师",
);

const stats = computed(() => {
  const s = summary.value;
  return [
    { label: "在读学员", value: s ? s.students_total : "—" },
    { label: "待批阅", value: s ? s.submissions_pending : "—" },
    { label: "待审核预约", value: s ? s.bookings_pending : "—" },
    { label: "课时预警", value: s ? s.low_balance_students : "—" },
  ];
});

const actions = computed<ActionItem[]>(() => {
  const list: ActionItem[] = [
    {
      key: "tasks",
      title: "任务与作业",
      desc: "新建、发布、逐份批阅",
      icon: "作",
      primary: true,
      badge: summary.value?.submissions_pending,
    },
    { key: "reviews", title: "待批阅", desc: "按提交时间处理", icon: "批" },
    { key: "students", title: "学员档案", desc: "分组、跟进、回访", icon: "学" },
    { key: "guardians", title: "绑定审批", desc: "家长申请与解绑", icon: "家", ownerOnly: true },
    { key: "import", title: "导入学员", desc: "Excel 批量导入", icon: "导", ownerOnly: true },
    { key: "staff", title: "成员管理", desc: "老师账号与权限", icon: "员", ownerOnly: true },
  ];
  return list.filter((item) => !item.ownerOnly || session.isOwner);
});

async function load(): Promise<void> {
  if (!session.isTeacher) {
    uni.reLaunch({ url: "/pkg-teacher/pages/login/index" });
    return;
  }
  loading.value = true;
  errorText.value = "";
  try {
    const ok = await session.refresh();
    if (!ok && !session.token) {
      uni.reLaunch({ url: "/pkg-teacher/pages/login/index" });
      return;
    }
    const res = await request<{ summary: Summary }>("/api/v1/dashboard/summary", "GET");
    summary.value = res.summary;
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      session.clear();
      uni.reLaunch({ url: "/pkg-teacher/pages/login/index" });
      return;
    }
    errorText.value = err instanceof ApiError ? err.message : "加载失败，请稍后重试";
  } finally {
    loading.value = false;
  }
}

onShow(() => {
  void load();
});

function go(key: ActionKey): void {
  const routes: Record<ActionKey, string> = {
    tasks: "/pkg-teacher/pages/tasks/index",
    reviews: "/pkg-teacher/pages/reviews/index",
    students: "/pkg-teacher/pages/students/index",
    guardians: "/pkg-teacher/pages/guardians/index",
    import: "/pkg-teacher/pages/import/index",
    staff: "/pkg-teacher/pages/staff/index",
  };
  uni.navigateTo({ url: routes[key] });
}

function logout(): void {
  session.clear();
  uni.reLaunch({ url: "/pages/index/index" });
}
</script>

<template>
  <view class="page">
    <view class="header">
      <view class="header-main">
        <text class="eyebrow">教师工作台</text>
        <text class="title">{{ session.staffName || "老师" }}</text>
      </view>
      <text class="role-chip">{{ roleLabel }}</text>
    </view>

    <view v-if="errorText" class="error">{{ errorText }}</view>
    <view v-else-if="loading && !summary" class="hint">正在同步…</view>

    <view class="stats">
      <view v-for="item in stats" :key="item.label" class="stat">
        <text class="stat-value">{{ item.value }}</text>
        <text class="stat-label">{{ item.label }}</text>
      </view>
    </view>

    <view class="section-head">
      <text class="section-title">常用操作</text>
      <text class="section-hint">点按进入</text>
    </view>

    <view class="actions">
      <view
        v-for="item in actions"
        :key="item.key"
        class="action"
        :class="{ 'action--primary': item.primary }"
        hover-class="action--pressed"
        @click="go(item.key)"
      >
        <view class="action-icon">{{ item.icon }}</view>
        <view class="action-copy">
          <text class="action-title">{{ item.title }}</text>
          <text class="action-desc">{{ item.desc }}</text>
        </view>
        <text v-if="item.badge" class="action-badge">{{ item.badge }}</text>
      </view>
    </view>

    <button class="logout" hover-class="logout--pressed" @click="logout">退出登录</button>
  </view>
</template>

<style lang="scss" scoped>
@import "@/styles/tokens.scss";

.page {
  min-height: 100vh;
  padding: 40rpx $hm-space 64rpx;
  box-sizing: border-box;
  background: $hm-bg;
  padding-bottom: calc(64rpx + env(safe-area-inset-bottom));
}

.header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20rpx;
}

.header-main {
  display: flex;
  flex-direction: column;
  gap: 6rpx;
}

.eyebrow {
  font-size: 22rpx;
  color: $hm-muted;
  letter-spacing: 2rpx;
}

.title {
  font-size: 44rpx;
  line-height: 1.25;
  font-weight: 700;
  color: $hm-text;
}

.role-chip {
  flex-shrink: 0;
  margin-top: 6rpx;
  padding: 8rpx 20rpx;
  border-radius: 999rpx;
  background: rgba(13, 148, 136, 0.1);
  color: $hm-primary;
  font-size: 22rpx;
  font-weight: 600;
}

.hint,
.error {
  display: block;
  margin-top: 20rpx;
  font-size: 24rpx;
  color: $hm-muted;
}

.error {
  color: #b91c1c;
}

.stats {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16rpx;
  margin-top: 28rpx;
}

.stat {
  display: flex;
  flex-direction: column;
  gap: 4rpx;
  padding: 22rpx 24rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  border-radius: 20rpx;
}

.stat-value {
  font-size: 38rpx;
  line-height: 1.1;
  font-weight: 700;
  color: $hm-text;
}

.stat-label {
  font-size: 22rpx;
  color: $hm-muted;
}

.section-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin: 40rpx 4rpx 18rpx;
}

.section-title {
  font-size: 28rpx;
  font-weight: 700;
  color: $hm-text;
}

.section-hint {
  font-size: 22rpx;
  color: $hm-muted;
}

.actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16rpx;
}

.action {
  position: relative;
  display: flex;
  align-items: center;
  gap: 16rpx;
  min-height: 132rpx;
  padding: 22rpx 20rpx;
  box-sizing: border-box;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  border-radius: 22rpx;
  transition: transform 0.12s ease;
}

.action--pressed {
  transform: scale(0.98);
}

.action--primary {
  grid-column: 1 / -1;
  min-height: 148rpx;
  background: $hm-primary;
  border-color: $hm-primary;
}

.action-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 64rpx;
  height: 64rpx;
  border-radius: 18rpx;
  background: rgba(13, 148, 136, 0.1);
  color: $hm-primary;
  font-size: 26rpx;
  font-weight: 700;
}

.action--primary .action-icon {
  background: rgba(255, 255, 255, 0.16);
  color: #ffffff;
}

.action-copy {
  display: flex;
  flex-direction: column;
  gap: 4rpx;
  min-width: 0;
}

.action-title {
  font-size: 28rpx;
  font-weight: 600;
  color: $hm-text;
}

.action-desc {
  font-size: 21rpx;
  line-height: 1.35;
  color: $hm-muted;
}

.action--primary .action-title {
  color: #ffffff;
}

.action--primary .action-desc {
  color: rgba(255, 255, 255, 0.78);
}

.action-badge {
  position: absolute;
  top: 16rpx;
  right: 16rpx;
  min-width: 32rpx;
  height: 32rpx;
  padding: 0 8rpx;
  box-sizing: border-box;
  border-radius: 16rpx;
  background: $hm-accent;
  color: #ffffff;
  font-size: 20rpx;
  line-height: 32rpx;
  text-align: center;
}

.logout {
  margin-top: 36rpx;
  height: 80rpx;
  line-height: 80rpx;
  border-radius: 40rpx;
  background: transparent;
  border: 1rpx solid $hm-border;
  color: $hm-muted;
  font-size: 26rpx;
}

.logout--pressed {
  background: rgba(0, 0, 0, 0.03);
}
</style>
