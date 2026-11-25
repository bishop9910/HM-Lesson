<script setup lang="ts">
import { ref } from "vue";
import { onShow } from "@dcloudio/uni-app";

import { ApiError, request } from "@/utils/request";

interface GroupCount {
  code: string;
  name: string;
  count: number;
}

interface Delivery {
  tasks: number;
  assignments: number;
  submissions: number;
  overdue: number;
  pending_review: number;
  submit_rate: number;
}

interface OneOnOne {
  pending_bookings: number;
  week_sessions: number;
  month_consumed: number;
}

interface Todo {
  follow_ups_due: number;
  overdue_approved: number;
  unconfirmed: number;
  low_balance: number;
}

interface Board {
  groups: GroupCount[];
  new_students: number;
  new_leads: number;
  converted: number;
  conversion_rate: number;
  delivery: Delivery;
  one_on_one: OneOnOne;
  todo: Todo;
  from: string;
  to: string;
  generated_at: string;
}

const loading = ref(true);
const error = ref("");
const board = ref<Board | null>(null);

function percent(value: number): string {
  return `${Math.round(value * 1000) / 10}%`;
}

function shortDate(iso: string): string {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return `${date.getMonth() + 1}月${date.getDate()}日`;
}

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    const res = await request<{ board: Board }>("/api/v1/dashboard/board", "GET");
    board.value = res.board;
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : "读取看板失败";
  } finally {
    loading.value = false;
  }
}

function go(page: string): void {
  uni.navigateTo({ url: page });
}

onShow(() => {
  void load();
});
</script>

<template>
  <view class="page">
    <view class="head">
      <text class="title">看板</text>
      <text class="sub">
        区间 {{ shortDate(board?.from ?? "") }} 至 {{ shortDate(board?.to ?? "") }} · 数据 60 秒缓存
      </text>
    </view>

    <view v-if="loading" class="hint">加载中…</view>
    <view v-else-if="error" class="hint" @tap="load">{{ error }}，点我重试</view>

    <template v-else-if="board">
      <view class="group">
        <text class="section">客户结构</text>
        <view class="cards">
          <view v-for="group in board.groups" :key="group.code" class="stat">
            <text class="stat-num">{{ group.count }}</text>
            <text class="stat-label">{{ group.name }}</text>
          </view>
        </view>
        <view class="facts">
          <text class="fact">本月新增 {{ board.new_students }}</text>
          <text class="fact">新增意向/访客 {{ board.new_leads }}</text>
          <text class="fact">转在读 {{ board.converted }}</text>
          <text class="fact">转化率 {{ percent(board.conversion_rate) }}</text>
        </view>
      </view>

      <view class="group">
        <text class="section">交付质量</text>
        <view class="cards">
          <view class="stat">
            <text class="stat-num">{{ board.delivery.tasks }}</text>
            <text class="stat-label">任务</text>
          </view>
          <view class="stat">
            <text class="stat-num">{{ board.delivery.assignments }}</text>
            <text class="stat-label">下发人次</text>
          </view>
          <view class="stat">
            <text class="stat-num">{{ percent(board.delivery.submit_rate) }}</text>
            <text class="stat-label">提交率</text>
          </view>
          <view class="stat">
            <text class="stat-num warn">{{ board.delivery.overdue }}</text>
            <text class="stat-label">逾期未交</text>
          </view>
          <view class="stat">
            <text class="stat-num">{{ board.delivery.pending_review }}</text>
            <text class="stat-label">待批阅</text>
          </view>
        </view>
      </view>

      <view class="group">
        <text class="section">一对一运营</text>
        <view class="cards">
          <view class="stat" @tap="go('/pkg-teacher/pages/bookings/index')">
            <text class="stat-num">{{ board.one_on_one.pending_bookings }}</text>
            <text class="stat-label">待审核预约</text>
          </view>
          <view class="stat">
            <text class="stat-num">{{ board.one_on_one.week_sessions }}</text>
            <text class="stat-label">本周排课</text>
          </view>
          <view class="stat">
            <text class="stat-num">{{ board.one_on_one.month_consumed }}</text>
            <text class="stat-label">本月消课</text>
          </view>
        </view>
      </view>

      <view class="group">
        <text class="section">待办</text>
        <view class="todo" @tap="go('/pkg-teacher/pages/students/index')">
          <text class="todo-label">待回访学员</text>
          <text class="todo-num">{{ board.todo.follow_ups_due }}</text>
        </view>
        <view class="todo" @tap="go('/pkg-teacher/pages/bookings/index')">
          <text class="todo-label">已通过但时间已过未完成</text>
          <text class="todo-num warn">{{ board.todo.overdue_approved }}</text>
        </view>
        <view class="todo" @tap="go('/pkg-teacher/pages/notices/index')">
          <text class="todo-label">未确认回执</text>
          <text class="todo-num">{{ board.todo.unconfirmed }}</text>
        </view>
        <view class="todo" @tap="go('/pkg-teacher/pages/lessons/index')">
          <text class="todo-label">余额 ≤ 2 课时</text>
          <text class="todo-num warn">{{ board.todo.low_balance }}</text>
        </view>
      </view>

      <view class="btn" hover-class="press" @tap="load">刷新</view>
    </template>
  </view>
</template>

<style lang="scss" scoped>
@import "@/styles/tokens.scss";

.page {
  min-height: 100vh;
  padding: 24rpx 24rpx 80rpx;
  background: $hm-bg;
}

.head {
  padding: 8rpx 4rpx 20rpx;
}

.title {
  display: block;
  font-size: 44rpx;
  font-weight: 600;
  color: $hm-ink;
}

.sub {
  display: block;
  margin-top: 6rpx;
  font-size: 24rpx;
  color: $hm-muted;
}

.section {
  display: block;
  margin: 24rpx 0 12rpx;
  font-size: 28rpx;
  font-weight: 600;
  color: $hm-ink;
}

.cards {
  display: flex;
  flex-wrap: wrap;
}

.stat {
  width: 200rpx;
  box-sizing: border-box;
  margin: 0 14rpx 14rpx 0;
  padding: 20rpx 18rpx;
  border-radius: $hm-radius;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

.stat-num {
  display: block;
  font-size: 40rpx;
  font-weight: 600;
  color: $hm-primary-strong;
}

.stat-num.warn {
  color: #b91c1c;
}

.stat-label {
  display: block;
  margin-top: 6rpx;
  font-size: 22rpx;
  color: $hm-muted;
}

.facts {
  display: flex;
  flex-wrap: wrap;
  padding: 14rpx 18rpx;
  border-radius: $hm-radius;
  background: rgba(251, 246, 234, 0.7);
  border: 1rpx solid $hm-border;
}

.fact {
  margin-right: 22rpx;
  font-size: 24rpx;
  color: $hm-muted;
}

.todo {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20rpx 22rpx;
  margin-bottom: 12rpx;
  border-radius: 14rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

.todo-label {
  font-size: 28rpx;
  color: $hm-ink;
}

.todo-num {
  font-size: 30rpx;
  font-weight: 600;
  color: $hm-primary-strong;
}

.todo-num.warn {
  color: #b91c1c;
}

.btn {
  margin-top: 20rpx;
  height: 76rpx;
  line-height: 76rpx;
  border-radius: 16rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  color: $hm-ink;
  font-size: 28rpx;
  text-align: center;
}

.hint {
  padding: 60rpx 0;
  text-align: center;
  color: $hm-muted;
  font-size: 26rpx;
}
</style>
