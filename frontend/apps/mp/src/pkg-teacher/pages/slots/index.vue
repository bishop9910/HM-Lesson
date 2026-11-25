<script setup lang="ts">
import { computed, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";

interface Slot {
  id: string;
  staff_name: string;
  start_at: string;
  end_at: string;
  status: string;
  note: string;
  booking_id: string;
  student_name: string;
  booking_status: string;
}

const slots = ref<Slot[]>([]);
const loading = ref(true);
const error = ref("");
const saving = ref(false);

const today = new Date();
const startDate = ref(fmtDate(today));
const startTime = ref("19:00");
const endTime = ref("20:00");
const note = ref("");
const weekdays = ref<number[]>([]);
const weeks = ref(4);

const WEEKDAY_LABELS = ["一", "二", "三", "四", "五", "六", "日"];

const statusLabel = computed<Record<string, string>>(() => ({
  open: "可预约",
  locked: "已占用",
  completed: "已完成",
  cancelled: "已取消",
}));

function fmtDate(d: Date): string {
  const m = `${d.getMonth() + 1}`.padStart(2, "0");
  const day = `${d.getDate()}`.padStart(2, "0");
  return `${d.getFullYear()}-${m}-${day}`;
}

function fmt(iso: string): string {
  const d = new Date(iso);
  return `${d.getMonth() + 1}月${d.getDate()}日 ${`${d.getHours()}`.padStart(2, "0")}:${`${d.getMinutes()}`.padStart(2, "0")}`;
}

function isoOf(date: string, time: string): string {
  return new Date(`${date}T${time}:00+08:00`).toISOString();
}

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    const res = await request<{ items: Slot[] }>("/api/v1/slots", "GET");
    slots.value = res.items;
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : "加载失败";
  } finally {
    loading.value = false;
  }
}

async function createSlot(): Promise<void> {
  saving.value = true;
  try {
    await request("/api/v1/slots", "POST", {
      start_at: isoOf(startDate.value, startTime.value),
      end_at: isoOf(startDate.value, endTime.value),
      note: note.value.trim(),
    });
    uni.showToast({ title: "已开放时段", icon: "success" });
    note.value = "";
    await load();
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : "创建失败", icon: "none" });
  } finally {
    saving.value = false;
  }
}

function toggleWeekday(day: number): void {
  weekdays.value = weekdays.value.includes(day)
    ? weekdays.value.filter((item) => item !== day)
    : [...weekdays.value, day];
}

async function batchSlots(): Promise<void> {
  if (weekdays.value.length === 0) {
    uni.showToast({ title: "请先选择星期", icon: "none" });
    return;
  }
  saving.value = true;
  try {
    const res = await request<{ created: number }>("/api/v1/slots/batch", "POST", {
      start_at: isoOf(startDate.value, startTime.value),
      end_at: isoOf(startDate.value, endTime.value),
      weekdays: weekdays.value,
      weeks: weeks.value,
    });
    uni.showToast({ title: `生成了 ${res.created} 个时段`, icon: "success" });
    await load();
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : "批量开放失败", icon: "none" });
  } finally {
    saving.value = false;
  }
}

async function cancelSlot(slot: Slot): Promise<void> {
  const res = await uni.showModal({ title: "取消时段", content: `取消 ${fmt(slot.start_at)} 的时段？`, confirmText: "取消时段" });
  if (!res.confirm) return;
  try {
    await request(`/api/v1/slots/${slot.id}`, "DELETE");
    uni.showToast({ title: "已取消", icon: "success" });
    await load();
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : "取消失败", icon: "none" });
  }
}

function onDate(e: { detail: { value: string } }): void {
  startDate.value = e.detail.value;
}
function onStartTime(e: { detail: { value: string } }): void {
  startTime.value = e.detail.value;
}
function onEndTime(e: { detail: { value: string } }): void {
  endTime.value = e.detail.value;
}

onShow(load);
</script>

<template>
  <view class="page">
    <view class="head">
      <text class="title">可预约时段</text>
      <text class="sub">{{ slots.length }} 个</text>
    </view>

    <view class="card">
      <text class="card-title">开放新时段</text>
      <view class="form-row">
        <text class="label">日期</text>
        <picker mode="date" :value="startDate" @change="onDate">
          <text class="value">{{ startDate }}</text>
        </picker>
      </view>
      <view class="form-row">
        <text class="label">开始</text>
        <picker mode="time" :value="startTime" @change="onStartTime">
          <text class="value">{{ startTime }}</text>
        </picker>
        <text class="label tight">结束</text>
        <picker mode="time" :value="endTime" @change="onEndTime">
          <text class="value">{{ endTime }}</text>
        </picker>
      </view>
      <input v-model="note" class="input" placeholder="备注（可选，如：冲刺电磁感应）" />
      <button class="btn" :loading="saving" @tap="createSlot">开放单个时段</button>

      <view class="divider" />
      <text class="card-title">按周批量开放</text>
      <view class="chips">
        <text
          v-for="(label, index) in WEEKDAY_LABELS"
          :key="label"
          class="chip"
          :class="{ active: weekdays.includes(index + 1) }"
          @tap="toggleWeekday(index + 1)"
        >
          周{{ label }}
        </text>
      </view>
      <view class="form-row">
        <text class="label">连续</text>
        <input v-model.number="weeks" class="input inline" type="number" />
        <text class="label tight">周</text>
      </view>
      <button class="btn ghost" :loading="saving" @tap="batchSlots">按周生成</button>
    </view>

    <view v-if="loading" class="hint">加载中…</view>
    <view v-else-if="error" class="hint error" @tap="load">{{ error }}，点此重试</view>
    <view v-else-if="slots.length === 0" class="hint">还没有开放时段，家长暂时无法预约。</view>
    <view v-for="slot in slots" :key="slot.id" class="slot">
      <view class="row">
        <text class="when">{{ fmt(slot.start_at) }}</text>
        <text class="badge" :class="slot.status">{{ statusLabel[slot.status] ?? slot.status }}</text>
      </view>
      <text v-if="slot.note" class="note">{{ slot.note }}</text>
      <view class="row foot">
        <text class="who">{{ slot.student_name ? `${slot.student_name} 已预约` : "暂无人预约" }}</text>
        <text v-if="slot.status === 'open'" class="link" @tap="cancelSlot(slot)">取消时段</text>
      </view>
    </view>
  </view>
</template>

<style lang="scss">
@import "@/styles/tokens.scss";
.page { min-height: 100vh; background: $hm-bg; padding: 24rpx 24rpx 60rpx; }
.head { display: flex; justify-content: space-between; align-items: baseline; margin-bottom: 20rpx; }
.title { font-size: 34rpx; font-weight: 700; }
.sub { color: $hm-muted; font-size: 24rpx; }
.card { background: $hm-surface; border: 1rpx solid $hm-rule; border-radius: $hm-radius; padding: 24rpx; margin-bottom: 20rpx; }
.card-title { font-size: 27rpx; font-weight: 600; }
.form-row { display: flex; align-items: center; margin-top: 18rpx; }
.label { color: $hm-muted; font-size: 25rpx; width: 96rpx; }
.label.tight { width: auto; margin: 0 12rpx; }
.value { font-size: 27rpx; padding: 12rpx 20rpx; background: $hm-paper-deep; border-radius: 12rpx; }
.input { margin-top: 18rpx; background: $hm-paper-deep; border-radius: 12rpx; padding: 18rpx 20rpx; font-size: 26rpx; }
.input.inline { width: 120rpx; margin-top: 0; text-align: center; }
.btn { margin-top: 22rpx; background: $hm-primary; color: #fff; font-size: 27rpx; border-radius: 12rpx; }
.btn.ghost { background: transparent; color: $hm-primary; border: 1rpx solid $hm-primary; }
.divider { height: 1rpx; background: $hm-rule; margin: 28rpx 0 20rpx; }
.chips { display: flex; flex-wrap: wrap; margin-top: 14rpx; }
.chip { font-size: 24rpx; padding: 10rpx 22rpx; border-radius: 999rpx; border: 1rpx solid $hm-rule; margin: 0 12rpx 12rpx 0; color: $hm-muted; }
.chip.active { background: $hm-primary; border-color: $hm-primary; color: #fff; }
.slot { background: $hm-surface; border: 1rpx solid $hm-rule; border-radius: $hm-radius; padding: 22rpx; margin-bottom: 16rpx; }
.row { display: flex; justify-content: space-between; align-items: center; }
.when { font-size: 28rpx; font-weight: 600; }
.badge { font-size: 22rpx; color: $hm-muted; }
.badge.open { color: $hm-primary; }
.badge.locked { color: $hm-accent; }
.note { display: block; color: $hm-muted; font-size: 24rpx; margin-top: 8rpx; }
.foot { margin-top: 12rpx; }
.who { color: $hm-muted; font-size: 23rpx; }
.link { color: $hm-danger; font-size: 24rpx; }
.hint { color: $hm-muted; text-align: center; padding: 40rpx 0; font-size: 25rpx; }
.hint.error { color: $hm-danger; }
</style>
