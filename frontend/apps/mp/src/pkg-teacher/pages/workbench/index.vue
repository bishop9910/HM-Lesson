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
  if (!session.token) {
    uni.reLaunch({ url: "/pkg-teacher/pages/login/index" });
    return;
  }
  loading.value = true;
  errorText.value = "";
  try {
    // 先让服务端确认角色：本地 hm_role 可能缺失/过期，不能据此提前踢回登录。
    const ok = await session.refresh();
    if (ok && !session.isTeacher) {
      session.clear();
      uni.reLaunch({ url: "/pkg-teacher/pages/login/index" });
      return;
    }
    if (!session.token) {
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
  uni.reLaunch({ url: "/pages/home/index" });
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
  padding: 44rpx $hm-space calc(64rpx + env(safe-area-inset-bottom));
  box-sizing: border-box;
  background-color: $hm-paper;
  background-image: linear-gradient(
    to bottom,
    rgba(34, 48, 42, 0.035) 1rpx,
    transparent 1rpx
  );
  background-size: 100% 64rpx;
}

.header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}

.header-main {
  display: flex;
  flex-direction: column;
}

.eyebrow {
  margin-bottom: 10rpx;
  font-size: 22rpx;
  letter-spacing: 6rpx;
  color: $hm-primary-strong;
}

.title {
  font-family: $hm-serif;
  font-size: 48rpx;
  line-height: 1.2;
  font-weight: 700;
  color: $hm-ink;
}

.role-chip {
  flex-shrink: 0;
  margin-top: 8rpx;
  padding: 6rpx 18rpx;
  border: 1rpx solid $hm-rule;
  border-radius: 6rpx;
  background: rgba(255, 255, 255, 0.4);
  color: rgba(34, 48, 42, 0.62);
  font-size: 21rpx;
  letter-spacing: 2rpx;
}

.hint,
.error {
  display: block;
  margin-top: 22rpx;
  font-size: 24rpx;
  color: rgba(34, 48, 42, 0.55);
}

.error {
  color: #a33a2b;
}

.stats {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  margin-top: 30rpx;
  border-top: 1rpx solid $hm-rule;
}

.stat {
  display: flex;
  flex-direction: column;
  width: 49%;
  box-sizing: border-box;
  padding: 24rpx 6rpx 22rpx;
  border-bottom: 1rpx solid $hm-rule;
}

.stat-value {
  font-family: $hm-serif;
  font-size: 42rpx;
  line-height: 1.1;
  font-weight: 700;
  color: $hm-ink;
}

.stat-label {
  margin-top: 6rpx;
  font-size: 22rpx;
  color: rgba(34, 48, 42, 0.55);
}

.section-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin: 42rpx 2rpx 18rpx;
}

.section-title {
  font-family: $hm-serif;
  font-size: 30rpx;
  font-weight: 700;
  letter-spacing: 2rpx;
  color: $hm-ink;
}

.section-hint {
  font-size: 21rpx;
  color: rgba(34, 48, 42, 0.45);
}

.actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
}

.action {
  position: relative;
  display: flex;
  align-items: center;
  width: 48.5%;
  min-height: 124rpx;
  margin-bottom: 16rpx;
  padding: 20rpx 18rpx;
  box-sizing: border-box;
  background: rgba(255, 255, 255, 0.38);
  border: 1rpx solid $hm-rule;
  border-radius: 10rpx;
  transition: transform 0.12s ease;
}

.action--pressed {
  transform: scale(0.985);
  background: rgba(255, 255, 255, 0.62);
}

.action--primary {
  width: 100%;
  min-height: 136rpx;
  background: $hm-primary-strong;
  border-color: $hm-primary-strong;
}

.action-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 54rpx;
  height: 54rpx;
  margin-right: 14rpx;
  border: 1rpx solid rgba(34, 48, 42, 0.28);
  border-radius: 4rpx;
  color: $hm-primary-strong;
  font-family: $hm-serif;
  font-size: 25rpx;
  font-weight: 700;
}

.action--primary .action-icon {
  border-color: rgba(247, 243, 232, 0.5);
  color: #f7f3e8;
}

.action-copy {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.action-title {
  font-size: 27rpx;
  font-weight: 600;
  color: $hm-ink;
}

.action-desc {
  margin-top: 4rpx;
  font-size: 20rpx;
  line-height: 1.35;
  color: rgba(34, 48, 42, 0.55);
}

.action--primary .action-title {
  color: #f7f3e8;
}

.action--primary .action-desc {
  color: rgba(247, 243, 232, 0.72);
}

.action-badge {
  position: absolute;
  top: 12rpx;
  right: 12rpx;
  min-width: 30rpx;
  height: 30rpx;
  padding: 0 8rpx;
  box-sizing: border-box;
  border-radius: 15rpx;
  background: $hm-accent;
  color: #ffffff;
  font-size: 19rpx;
  line-height: 30rpx;
  text-align: center;
}

.logout {
  margin-top: 36rpx;
  height: 76rpx;
  line-height: 76rpx;
  background: transparent;
  border: 0;
  color: rgba(34, 48, 42, 0.5);
  font-size: 24rpx;
}

.logout::after {
  border: 0;
}

.logout--pressed {
  opacity: 0.7;
}
</style>
