<script setup lang="ts">
import { computed, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";

interface Student {
  id: string;
  name: string;
  grade: string;
}

interface Slot {
  id: string;
  staff_name: string;
  start_at: string;
  end_at: string;
  status: string;
  note: string;
  student_name: string;
}

interface Booking {
  id: string;
  slot_id: string;
  student_id: string;
  student_name: string;
  status: string;
  request_note: string;
  review_note: string;
  start_at: string;
  end_at: string;
  submitted_at: string;
}

const students = ref<Student[]>([]);
const slots = ref<Slot[]>([]);
const bookings = ref<Booking[]>([]);
const activeStudent = ref("");
const loading = ref(true);
const error = ref("");
const submitting = ref("");

const statusLabel: Record<string, string> = {
  pending: "待审核",
  approved: "已通过",
  rejected: "已驳回",
  completed: "已完成",
  cancelled: "已撤回",
  no_show: "未到课",
};

const currentStudent = computed(() => students.value.find((item) => item.id === activeStudent.value));

function fmt(iso: string): string {
  const d = new Date(iso);
  const week = ["日", "一", "二", "三", "四", "五", "六"][d.getDay()];
  return `${d.getMonth() + 1}月${d.getDate()}日 周${week} ${`${d.getHours()}`.padStart(2, "0")}:${`${d.getMinutes()}`.padStart(2, "0")}`;
}

function timeRange(slot: Slot): string {
  const end = new Date(slot.end_at);
  return `${fmt(slot.start_at)} – ${`${end.getHours()}`.padStart(2, "0")}:${`${end.getMinutes()}`.padStart(2, "0")}`;
}

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    const [home, slotRes, bookingRes] = await Promise.all([
      request<{ students: Student[] }>("/api/v1/portal/home", "GET", undefined, "guardian"),
      request<{ items: Slot[] }>("/api/v1/portal/slots", "GET", undefined, "guardian"),
      request<{ items: Booking[] }>("/api/v1/portal/bookings", "GET", undefined, "guardian"),
    ]);
    students.value = home.students;
    slots.value = slotRes.items;
    bookings.value = bookingRes.items;
    if (!students.value.some((item) => item.id === activeStudent.value)) {
      activeStudent.value = students.value.length > 0 ? students.value[0].id : "";
    }
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      uni.removeStorageSync("hm_guardian_token");
      uni.reLaunch({ url: "/pages/guardian-login/index" });
      return;
    }
    error.value = e instanceof ApiError ? e.message : "加载失败";
  } finally {
    loading.value = false;
  }
}

function pickStudent(id: string): void {
  activeStudent.value = id;
}

async function book(slot: Slot): Promise<void> {
  if (!activeStudent.value) {
    uni.showToast({ title: "请先选择学员", icon: "none" });
    return;
  }
  const res = await uni.showModal({
    title: "预约确认",
    editable: true,
    placeholderText: "想重点讲什么？（可选）",
    confirmText: "提交预约",
  });
  if (!res.confirm) return;
  submitting.value = slot.id;
  try {
    await request("/api/v1/portal/bookings", "POST", {
      student_id: activeStudent.value,
      slot_id: slot.id,
      request_note: (res.content ?? "").trim(),
    }, "guardian");
    uni.showToast({ title: "已提交，等老师确认", icon: "success" });
    await load();
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : "预约失败", icon: "none" });
  } finally {
    submitting.value = "";
  }
}

async function cancel(booking: Booking): Promise<void> {
  const res = await uni.showModal({
    title: "撤回预约",
    content: `撤回 ${fmt(booking.start_at)} 的预约？`,
    confirmText: "撤回",
  });
  if (!res.confirm) return;
  try {
    await request(`/api/v1/portal/bookings/${booking.id}/cancel`, "POST", {}, "guardian");
    uni.showToast({ title: "已撤回", icon: "success" });
    await load();
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : "撤回失败", icon: "none" });
  }
}

onShow(load);
</script>

<template>
  <view class="page">
    <view v-if="loading" class="hint">加载中…</view>
    <view v-else-if="error" class="hint error" @tap="load">{{ error }}，点此重试</view>
    <template v-else>
      <view v-if="students.length === 0" class="hint">还没有绑定学员，请先在首页提交绑定申请。</view>
      <template v-else>
        <view v-if="students.length > 1" class="students">
          <text
            v-for="student in students"
            :key="student.id"
            class="student-chip"
            :class="{ active: student.id === activeStudent }"
            @tap="pickStudent(student.id)"
          >
            {{ student.name }}<text v-if="student.grade"> · {{ student.grade }}</text>
          </text>
        </view>

        <view class="head">
          <text class="title">可预约时段</text>
          <text class="sub">{{ currentStudent?.name }} 预约</text>
        </view>
        <view v-if="slots.length === 0" class="hint small">老师暂时没有开放时段，稍后再来看看。</view>
        <view v-for="slot in slots" :key="slot.id" class="card">
          <view class="row">
            <text class="when">{{ timeRange(slot) }}</text>
            <text v-if="slot.student_name" class="taken">已被预约</text>
          </view>
          <text v-if="slot.note" class="note">{{ slot.note }}</text>
          <text class="teacher">{{ slot.staff_name }}</text>
          <button
            v-if="!slot.student_name"
            class="btn"
            :loading="submitting === slot.id"
            @tap="book(slot)"
          >
            预约这个时段
          </button>
        </view>

        <view class="head">
          <text class="title">我的预约</text>
          <text class="sub">{{ bookings.length }} 条</text>
        </view>
        <view v-if="bookings.length === 0" class="hint small">还没有预约记录。</view>
        <view v-for="item in bookings" :key="item.id" class="card">
          <view class="row">
            <text class="when">{{ fmt(item.start_at) }}</text>
            <text class="badge" :class="item.status">{{ statusLabel[item.status] ?? item.status }}</text>
          </view>
          <text class="teacher">{{ item.student_name }}</text>
          <text v-if="item.request_note" class="note">诉求：{{ item.request_note }}</text>
          <text v-if="item.review_note" class="note">老师意见：{{ item.review_note }}</text>
          <text v-if="item.status === 'pending'" class="link" @tap="cancel(item)">撤回预约</text>
        </view>
      </template>
    </template>
  </view>
</template>

<style lang="scss">
@import "@/styles/tokens.scss";
.page { min-height: 100vh; background: $hm-bg; padding: 24rpx 24rpx 60rpx; }
.students { display: flex; flex-wrap: wrap; margin-bottom: 20rpx; }
.student-chip { font-size: 24rpx; padding: 10rpx 22rpx; border-radius: 999rpx; border: 1rpx solid $hm-rule; margin: 0 12rpx 12rpx 0; color: $hm-muted; }
.student-chip.active { background: $hm-primary; border-color: $hm-primary; color: #fff; }
.head { display: flex; justify-content: space-between; align-items: baseline; margin: 10rpx 0 16rpx; }
.title { font-size: 31rpx; font-weight: 700; }
.sub { color: $hm-muted; font-size: 23rpx; }
.card { background: $hm-surface; border: 1rpx solid $hm-rule; border-radius: $hm-radius; padding: 24rpx; margin-bottom: 16rpx; }
.row { display: flex; justify-content: space-between; align-items: center; }
.when { font-size: 27rpx; font-weight: 600; }
.taken { color: $hm-muted; font-size: 22rpx; }
.badge { font-size: 22rpx; color: $hm-muted; }
.badge.pending { color: $hm-accent; }
.badge.approved { color: $hm-primary; }
.badge.rejected { color: $hm-danger; }
.note { display: block; color: #475569; font-size: 24rpx; margin-top: 10rpx; line-height: 1.6; }
.teacher { display: block; color: $hm-muted; font-size: 23rpx; margin-top: 6rpx; }
.btn { margin-top: 18rpx; background: $hm-primary; color: #fff; font-size: 26rpx; border-radius: 12rpx; }
.link { display: block; color: $hm-danger; font-size: 24rpx; margin-top: 14rpx; }
.hint { color: $hm-muted; text-align: center; padding: 40rpx 0; font-size: 25rpx; }
.hint.small { padding: 24rpx 0; font-size: 23rpx; }
.hint.error { color: $hm-danger; }
</style>
