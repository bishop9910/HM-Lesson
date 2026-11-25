<script setup lang="ts">
import { computed, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";
import { useSessionStore } from "@/stores/session";

interface PendingGuardian {
  id: string;
  phone: string;
  name: string;
  status: string;
  created_at: string;
}
interface GuardianAccount {
  id: string;
  phone: string;
  name: string;
  status: string;
  has_password: boolean;
  last_login_at?: string;
  created_at: string;
  student_ids: string[];
  student_names: string[];
  student_count: number;
}
interface Student {
  id: string;
  name: string;
  grade: string;
}

const session = useSessionStore();
const pending = ref<PendingGuardian[]>([]);
const accounts = ref<GuardianAccount[]>([]);
const students = ref<Student[]>([]);
const selected = ref<Record<string, string>>({});
const loading = ref(false);
const errorText = ref("");
const tab = ref<"pending" | "accounts">("pending");
const statusFilter = ref("");

const statusLabels: Record<string, string> = {
  pending: "待审批",
  active: "生效中",
  disabled: "已停用",
  rejected: "已驳回",
};

const studentNames = computed(() => students.value.map((s) => `${s.name}（${s.grade}）`));

const filteredAccounts = computed(() =>
  statusFilter.value ? accounts.value.filter((a) => a.status === statusFilter.value) : accounts.value,
);

const cleanupCount = computed(
  () => accounts.value.filter((a) => a.status !== "pending" && a.student_count === 0).length,
);

async function load(): Promise<void> {
  if (!session.isOwner) {
    uni.reLaunch({ url: "/pkg-teacher/pages/workbench/index" });
    return;
  }
  loading.value = true;
  errorText.value = "";
  try {
    const [pend, list, accs] = await Promise.all([
      request<{ items: PendingGuardian[] }>("/api/v1/guardians/pending", "GET"),
      request<{ items: Student[] }>("/api/v1/students?page=1&page_size=50", "GET"),
      request<{ items: GuardianAccount[] }>("/api/v1/guardians", "GET"),
    ]);
    pending.value = pend.items;
    students.value = list.items;
    accounts.value = accs.items;
  } catch (err) {
    errorText.value = err instanceof ApiError ? err.message : "加载失败";
  } finally {
    loading.value = false;
  }
}

onShow(() => {
  void load();
});

function pickStudent(guardianId: string, index: number): void {
  const student = students.value[index];
  if (student) selected.value = { ...selected.value, [guardianId]: student.id };
}

function onPick(guardianId: string, e: { detail: { value: number } }): void {
  pickStudent(guardianId, e.detail.value);
}

function studentName(id: string): string {
  return students.value.find((s) => s.id === id)?.name ?? "选择学员";
}

async function approveWith(item: PendingGuardian, password: string): Promise<void> {
  const studentId = selected.value[item.id];
  if (!studentId) {
    uni.showToast({ title: "先选择学员", icon: "none" });
    return;
  }
  try {
    const res = await request<{ status: string; initial_password?: string }>(
      `/api/v1/guardians/${item.id}/approve`,
      "POST",
      { student_id: studentId, password },
    );
    await load();
    if (res.initial_password) {
      uni.showModal({
        title: "已通过，初始密码",
        content: `请把初始密码告诉家长：${res.initial_password}\n家长登录后可在门户里改密码。`,
        showCancel: false,
      });
    } else {
      uni.showToast({ title: "已通过", icon: "success" });
    }
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "操作失败", icon: "none" });
  }
}

function approve(item: PendingGuardian): void {
  if (!selected.value[item.id]) {
    uni.showToast({ title: "先选择学员", icon: "none" });
    return;
  }
  uni.showModal({
    title: "设置家长初始密码",
    editable: true,
    placeholderText: "留空则自动生成 8 位密码",
    success: (res) => {
      if (!res.confirm) return;
      void approveWith(item, (res.content ?? "").trim());
    },
  });
}

function reject(item: PendingGuardian): void {
  uni.showModal({
    title: "驳回绑定",
    content: `确认驳回 ${item.phone} 的绑定申请？`,
    success: async (res) => {
      if (!res.confirm) return;
      try {
        await request(`/api/v1/guardians/${item.id}/reject`, "POST", { reason: "信息无法核实" });
        uni.showToast({ title: "已驳回", icon: "success" });
        await load();
      } catch (err) {
        uni.showToast({ title: err instanceof ApiError ? err.message : "操作失败", icon: "none" });
      }
    },
  });
}

function resetPassword(item: GuardianAccount): void {
  uni.showModal({
    title: `重置 ${item.phone} 的密码`,
    editable: true,
    placeholderText: "新密码（6~32 位）",
    success: async (res) => {
      if (!res.confirm) return;
      const password = (res.content ?? "").trim();
      if (password.length < 6) {
        uni.showToast({ title: "密码至少 6 位", icon: "none" });
        return;
      }
      try {
        await request(`/api/v1/guardians/${item.id}/password`, "POST", { password });
        uni.showToast({ title: "已重置", icon: "success" });
      } catch (err) {
        uni.showToast({ title: err instanceof ApiError ? err.message : "操作失败", icon: "none" });
      }
    },
  });
}

function remove(item: GuardianAccount): void {
  if (item.student_count > 0) {
    uni.showToast({ title: "仍绑定在读学员，先归档学员", icon: "none" });
    return;
  }
  uni.showModal({
    title: "删除家长账号",
    content: `确认删除 ${item.phone}？删除后该家长无法登录，历史通知一并清理。`,
    confirmColor: "#b91c1c",
    success: async (res) => {
      if (!res.confirm) return;
      try {
        await request(`/api/v1/guardians/${item.id}`, "DELETE");
        uni.showToast({ title: "已删除", icon: "success" });
        await load();
      } catch (err) {
        uni.showToast({ title: err instanceof ApiError ? err.message : "删除失败", icon: "none" });
      }
    },
  });
}

function switchTab(next: "pending" | "accounts"): void {
  tab.value = next;
}
</script>

<template>
  <view class="page">
    <text class="title">家长账号</text>
    <text class="subtitle">
      审批绑定、给家长设初始密码；没有在读学员的账号可以在这里清理。
    </text>

    <view class="tabs">
      <view class="tab" :class="{ active: tab === 'pending' }" @click="switchTab('pending')">
        待审批 {{ pending.length }}
      </view>
      <view class="tab" :class="{ active: tab === 'accounts' }" @click="switchTab('accounts')">
        全部账号 {{ accounts.length }}
      </view>
    </view>

    <view v-if="errorText" class="error">{{ errorText }}</view>

    <template v-if="tab === 'pending'">
      <view v-if="loading" class="hint">正在加载…</view>
      <view v-else-if="!pending.length" class="hint">暂无待审批申请。</view>
      <view v-for="item in pending" :key="item.id" class="card">
        <view class="card-top">
          <text class="phone">{{ item.phone }}</text>
          <text class="time">{{ item.created_at.slice(0, 10) }}</text>
        </view>
        <picker :range="studentNames" @change="onPick(item.id, $event)">
          <view class="select">
            <text class="select-label">关联学员</text>
            <text class="select-value">{{ studentName(selected[item.id] ?? "") }}</text>
          </view>
        </picker>
        <view class="actions">
          <button class="primary" @click="approve(item)">通过并设密码</button>
          <button class="ghost" @click="reject(item)">驳回</button>
        </view>
      </view>
    </template>

    <template v-else>
      <view class="filters">
        <view
          v-for="opt in [
            { key: '', label: '全部' },
            { key: 'active', label: '生效' },
            { key: 'disabled', label: '停用' },
            { key: 'pending', label: '待审批' },
            { key: 'rejected', label: '已驳回' },
          ]"
          :key="opt.key"
          class="chip"
          :class="{ active: statusFilter === opt.key }"
          @click="statusFilter = opt.key"
        >
          {{ opt.label }}
        </view>
      </view>
      <text v-if="cleanupCount" class="cleanup">有 {{ cleanupCount }} 个账号没有在读学员，可清理。</text>
      <view v-if="loading" class="hint">正在加载…</view>
      <view v-else-if="!filteredAccounts.length" class="hint">没有符合条件的家长账号。</view>
      <view v-for="item in filteredAccounts" :key="item.id" class="card">
        <view class="card-top">
          <text class="phone">{{ item.phone }}</text>
          <text class="badge" :class="item.status">{{ statusLabels[item.status] ?? item.status }}</text>
        </view>
        <text class="meta">
          {{ item.student_names.length ? `XXX 的家长：${item.student_names.join("、")}` : "未关联学员" }}
        </text>
        <text class="meta">
          {{ item.has_password ? "已设密码" : "未设密码" }} ·
          {{ item.last_login_at ? `上次登录 ${item.last_login_at.slice(5, 16).replace("T", " ")}` : "从未登录" }}
        </text>
        <view class="actions">
          <button class="ghost" @click="resetPassword(item)">重置密码</button>
          <button class="danger" @click="remove(item)">删除账号</button>
        </view>
      </view>
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

.title {
  display: block;
  font-size: 38rpx;
  font-weight: 700;
}

.subtitle {
  display: block;
  margin-top: 10rpx;
  font-size: 24rpx;
  color: $hm-muted;
  line-height: 1.5;
}

.tabs {
  display: flex;
  margin-top: 24rpx;
  padding: 6rpx;
  border-radius: 40rpx;
  background: $hm-paper-deep;
}

.tab {
  flex: 1;
  height: 64rpx;
  line-height: 64rpx;
  text-align: center;
  border-radius: 32rpx;
  font-size: 26rpx;
  color: $hm-muted;
}

.tab.active {
  background: $hm-surface;
  color: $hm-primary-strong;
  font-weight: 600;
}

.filters {
  display: flex;
  flex-wrap: wrap;
  margin-top: 20rpx;
}

.chip {
  margin: 0 12rpx 12rpx 0;
  padding: 10rpx 22rpx;
  border-radius: 30rpx;
  border: 1rpx solid $hm-border;
  font-size: 24rpx;
  color: $hm-muted;
}

.chip.active {
  background: $hm-primary;
  border-color: $hm-primary;
  color: #fff;
}

.cleanup {
  display: block;
  margin-top: 4rpx;
  font-size: 24rpx;
  color: $hm-accent;
}

.card {
  margin-top: 20rpx;
  padding: 24rpx;
  border-radius: $hm-radius;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

.card-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.phone {
  font-size: 32rpx;
  font-weight: 600;
}

.time {
  font-size: 22rpx;
  color: $hm-muted;
}

.badge {
  padding: 4rpx 16rpx;
  border-radius: 20rpx;
  font-size: 22rpx;
  background: $hm-paper-deep;
  color: $hm-muted;
}

.badge.active {
  background: rgba(15, 118, 110, 0.12);
  color: $hm-primary-strong;
}

.badge.disabled,
.badge.rejected {
  background: rgba(185, 28, 28, 0.1);
  color: #b91c1c;
}

.meta {
  display: block;
  margin-top: 10rpx;
  font-size: 24rpx;
  color: $hm-muted;
  line-height: 1.5;
}

.select {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 16rpx;
  padding: 20rpx 24rpx;
  border-radius: $hm-radius;
  background: $hm-bg;
  border: 1rpx solid $hm-border;
}

.select-label {
  font-size: 26rpx;
  color: $hm-muted;
}

.select-value {
  font-size: 26rpx;
  color: $hm-primary-strong;
}

.actions {
  display: flex;
  margin-top: 20rpx;
}

.primary,
.ghost,
.danger {
  flex: 1;
  height: 76rpx;
  line-height: 76rpx;
  border-radius: 38rpx;
  font-size: 26rpx;
}

.primary {
  background: $hm-primary;
  color: #fff;
}

.ghost {
  margin-left: 16rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  color: $hm-muted;
}

.danger {
  margin-left: 16rpx;
  background: $hm-surface;
  border: 1rpx solid rgba(185, 28, 28, 0.35);
  color: #b91c1c;
}

.actions > .ghost:first-child,
.actions > .primary:first-child,
.actions > .danger:first-child {
  margin-left: 0;
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
</style>
