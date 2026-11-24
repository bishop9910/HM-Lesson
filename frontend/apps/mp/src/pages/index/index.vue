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
const loadError = ref("");
const hasGuardian = ref(false);
const identityReady = ref(!session.token);

const sysInfo = uni.getSystemInfoSync();
const statusBarHeight = sysInfo.statusBarHeight ?? 20;

// 顶部留出状态栏 + 微信胶囊按钮的高度。
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
  const s = summary.value;
  if (!s) return 0;
  return s.submissions_pending + s.bookings_pending + s.low_balance_students;
});

async function loadSummary(): Promise<void> {
  if (!session.isTeacher) {
    summary.value = null;
    return;
  }
  loading.value = true;
  loadError.value = "";
  try {
    const res = await request<{ summary: Summary }>("/api/v1/dashboard/summary", "GET");
    summary.value = res.summary;
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      session.clear();
      summary.value = null;
    } else {
      loadError.value = "暂时取不到数据，稍后再试。";
    }
  } finally {
    loading.value = false;
  }
}

onShow(() => {
  hasGuardian.value = Boolean(uni.getStorageSync("hm_guardian_token"));
  void (async () => {
    if (session.isTeacher) {
      await session.refresh();
    }
    identityReady.value = true;
    await loadSummary();
  })();
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
  <view class="paper">
    <view
      class="topbar"
      :style="{ paddingTop: `${statusBarHeight}px`, height: `${topbarHeight}px` }"
    >
      <text class="brand">鹤鸣作业本</text>
    </view>

    <view class="page">
      <view v-if="session.isTeacher" class="masthead">
        <text class="kicker">教师案头</text>
        <text class="headline">{{ session.staffName || "老师" }}，今天</text>
        <text v-if="!identityReady" class="subline">正在核对身份。</text>
        <text v-else-if="loadError" class="subline">{{ loadError }}</text>
        <text v-else-if="summary" class="subline">
          有 {{ todoCount }} 项需要处理。
        </text>
        <text v-else class="subline">正在核对今天的课堂记录。</text>
      </view>

      <view v-else class="masthead">
        <text class="kicker">家长案头</text>
        <text class="headline">把作业交上来</text>
        <text class="subline">登录后查看孩子的作业、课时与上课记录。</text>
      </view>

      <view v-if="session.isTeacher" class="ledger">
        <view class="ledger-cell">
          <text class="ledger-value">{{ summary ? summary.submissions_pending : "—" }}</text>
          <text class="ledger-label">待批阅</text>
        </view>
        <view class="ledger-cell">
          <text class="ledger-value">{{ summary ? summary.bookings_pending : "—" }}</text>
          <text class="ledger-label">待审核</text>
        </view>
        <view class="ledger-cell">
          <text class="ledger-value">{{ summary ? summary.low_balance_students : "—" }}</text>
          <text class="ledger-label">课时提醒</text>
        </view>
      </view>

      <view v-else class="parent-slip">
        <text class="slip-title">{{ hasGuardian ? "已经登录家长端" : "还没有绑定学员" }}</text>
        <text class="slip-text">
          {{
            hasGuardian
              ? "点下面的按钮查看孩子的作业与课时。"
              : "用老师登记的家长手机号登录；没有记录时先提交绑定申请。"
          }}
        </text>
      </view>

      <view v-if="session.isTeacher" class="stack">
        <button class="row row--ink" hover-class="row--pressed" @click="goTeacher">
          <text class="row-title">进入工作台</text>
          <text class="row-note">任务、批阅、学员与课时</text>
          <view class="chev" />
        </button>
      </view>

      <view v-else class="stack stack--grid">
        <button class="row row--ink" hover-class="row--pressed" @click="goGuardian">
          <text class="row-title">{{ hasGuardian ? "查看孩子的作业" : "家长登录" }}</text>
          <text class="row-note">手机号登录，查看作业与课时</text>
          <view class="chev" />
        </button>
        <button class="row row--line" hover-class="row--pressed" @click="goBind">
          <text class="row-title">申请绑定学员</text>
          <text class="row-note">提交后由老师确认</text>
          <view class="chev" />
        </button>
        <button class="text-link" @click="goTeacher">我是老师</button>
      </view>

      <text v-if="loading" class="footnote">同步中…</text>
    </view>
  </view>
</template>

<style lang="scss" scoped>
@import "@/styles/tokens.scss";

.paper {
  min-height: 100vh;
  background-color: $hm-paper;
  background-image: linear-gradient(
    to bottom,
    rgba(34, 48, 42, 0.035) 1rpx,
    transparent 1rpx
  );
  background-size: 100% 64rpx;
}

.topbar {
  display: flex;
  align-items: center;
  box-sizing: border-box;
  padding-right: 220rpx;
  padding-left: $hm-space;
}

.brand {
  font-family: $hm-serif;
  font-size: 30rpx;
  font-weight: 600;
  letter-spacing: 1rpx;
  color: $hm-ink;
}

.page {
  padding: 10rpx $hm-space calc(56rpx + env(safe-area-inset-bottom));
  box-sizing: border-box;
}

.masthead {
  display: flex;
  flex-direction: column;
  padding: 26rpx 0 34rpx;
  border-bottom: 3rpx solid $hm-ink;
}

.kicker {
  margin-bottom: 12rpx;
  font-size: 22rpx;
  letter-spacing: 6rpx;
  color: $hm-primary-strong;
}

.headline {
  font-family: $hm-serif;
  font-size: 52rpx;
  line-height: 1.32;
  font-weight: 700;
  color: $hm-ink;
}

.subline {
  margin-top: 12rpx;
  max-width: 620rpx;
  font-size: 26rpx;
  line-height: 1.6;
  color: rgba(34, 48, 42, 0.66);
}

.ledger {
  display: flex;
  justify-content: space-between;
  margin-top: 26rpx;
  border-bottom: 1rpx solid $hm-rule;
}

.ledger-cell {
  display: flex;
  flex-direction: column;
  width: 32%;
  padding: 18rpx 0 22rpx;
}

.ledger-value {
  font-family: $hm-serif;
  font-size: 44rpx;
  line-height: 1.1;
  font-weight: 700;
  color: $hm-ink;
}

.ledger-label {
  margin-top: 4rpx;
  font-size: 22rpx;
  color: rgba(34, 48, 42, 0.55);
}

.parent-slip {
  display: flex;
  flex-direction: column;
  margin-top: 28rpx;
  padding: 24rpx 26rpx;
  border-left: 6rpx solid $hm-accent;
  background: rgba(255, 255, 255, 0.5);
}

.slip-title {
  font-size: 28rpx;
  font-weight: 600;
  color: $hm-ink;
}

.slip-text {
  margin-top: 8rpx;
  font-size: 24rpx;
  line-height: 1.6;
  color: rgba(34, 48, 42, 0.62);
}

.stack {
  display: flex;
  flex-direction: column;
  margin-top: 36rpx;
}

/* 两个一组，宽度撑满各自的格子；放不下时再换行。 */
.stack--grid {
  flex-direction: row;
  flex-wrap: wrap;
  justify-content: space-between;
}

.stack--grid .row {
  width: 48.5%;
  min-height: 148rpx;
  padding: 22rpx 56rpx 22rpx 20rpx;
}

.stack--grid .row-title {
  font-size: 27rpx;
}

.stack--grid .row-note {
  font-size: 21rpx;
}

.stack--grid .text-link {
  width: 100%;
  margin-top: 4rpx;
}

.row {
  position: relative;
  display: block;
  width: 100%;
  margin: 0 0 18rpx;
  padding: 24rpx 74rpx 24rpx 26rpx;
  box-sizing: border-box;
  border-radius: 14rpx;
  text-align: left;
  line-height: 1.35;
}

.row::after {
  border: 0;
}

.row--ink {
  background: $hm-primary-strong;
  border: 1rpx solid $hm-primary-strong;
}

.row--line {
  background: transparent;
  border: 1rpx solid $hm-rule;
}

.row--pressed {
  opacity: 0.88;
}

.row-title {
  display: block;
  font-size: 32rpx;
  font-weight: 600;
  letter-spacing: 1rpx;
  color: $hm-ink;
}

.row-note {
  display: block;
  margin-top: 6rpx;
  font-size: 23rpx;
  color: rgba(34, 48, 42, 0.58);
}

.row--ink .row-title {
  color: #f7f3e8;
}

.row--ink .row-note {
  color: rgba(247, 243, 232, 0.72);
}

.chev {
  position: absolute;
  top: 50%;
  right: 32rpx;
  width: 14rpx;
  height: 14rpx;
  margin-top: -7rpx;
  border-top: 3rpx solid currentColor;
  border-right: 3rpx solid currentColor;
  transform: rotate(45deg);
  opacity: 0.7;
}

.row--ink .chev {
  color: #f7f3e8;
}

.row--line .chev {
  color: $hm-ink;
}

.text-link {
  align-self: center;
  margin-top: 8rpx;
  padding: 12rpx 20rpx;
  background: transparent;
  border: 0;
  font-size: 24rpx;
  color: $hm-primary-strong;
}

.footnote {
  display: block;
  margin-top: 24rpx;
  font-size: 22rpx;
  color: rgba(34, 48, 42, 0.45);
}
</style>
