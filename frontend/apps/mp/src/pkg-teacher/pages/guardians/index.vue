<script setup lang="ts">
import { computed, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";
import { useSessionStore } from "@/stores/session";

interface Guardian {
  id: string;
  phone: string;
  name: string;
  created_at: string;
}
interface Student {
  id: string;
  name: string;
  grade: string;
}

const session = useSessionStore();
const guardians = ref<Guardian[]>([]);
const students = ref<Student[]>([]);
const selected = ref<Record<string, string>>({});
const loading = ref(false);
const errorText = ref("");

const studentNames = computed(() => students.value.map((s) => `${s.name}（${s.grade}）`));

async function load(): Promise<void> {
  if (!session.isOwner) {
    uni.reLaunch({ url: "/pkg-teacher/pages/workbench/index" });
    return;
  }
  loading.value = true;
  errorText.value = "";
  try {
    const [pending, list] = await Promise.all([
      request<{ items: Guardian[] }>("/api/v1/guardians/pending", "GET"),
      request<{ items: Student[] }>("/api/v1/students?page=1&page_size=50", "GET"),
    ]);
    guardians.value = pending.items;
    students.value = list.items;
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

async function approve(item: Guardian): Promise<void> {
  const studentId = selected.value[item.id];
  if (!studentId) {
    uni.showToast({ title: "先选择学员", icon: "none" });
    return;
  }
  try {
    await request(`/api/v1/guardians/${item.id}/approve`, "POST", { student_id: studentId });
    uni.showToast({ title: "已通过", icon: "success" });
    await load();
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "操作失败", icon: "none" });
  }
}

function reject(item: Guardian): void {
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

function studentName(id: string): string {
  return students.value.find((s) => s.id === id)?.name ?? "选择学员";
}
</script>

<template>
  <view class="page">
    <text class="title">家长绑定审批</text>
    <text class="subtitle">家长提交手机号后在这里关联学员；通过后家长即可登录查看。</text>

    <view v-if="errorText" class="error">{{ errorText }}</view>
    <view v-else-if="loading" class="hint">正在加载…</view>
    <view v-else-if="!guardians.length" class="hint">暂无待审批申请。</view>

    <view v-for="item in guardians" :key="item.id" class="card">
      <view class="card-top">
        <text class="phone">{{ item.phone }}</text>
        <text class="time">{{ item.created_at.slice(0, 10) }}</text>
      </view>
      <picker
        :range="studentNames"
        @change="onPick(item.id, $event)"
      >
        <view class="select">
          <text class="select-label">关联学员</text>
          <text class="select-value">{{ studentName(selected[item.id] ?? "") }}</text>
        </view>
      </picker>
      <view class="actions">
        <button class="primary" @click="approve(item)">通过</button>
        <button class="ghost" @click="reject(item)">驳回</button>
      </view>
    </view>
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
}

.phone {
  font-size: 32rpx;
  font-weight: 600;
}

.time {
  font-size: 22rpx;
  color: $hm-muted;
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
  gap: 16rpx;
  margin-top: 20rpx;
}

.primary,
.ghost {
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
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  color: $hm-muted;
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
