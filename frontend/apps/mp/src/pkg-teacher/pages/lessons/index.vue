<script setup lang="ts">
import { ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";

interface LessonAccount {
  student_id: string;
  student_name: string;
  grade: string;
  purchased_hours: number;
  used_hours: number;
  balance_hours: number;
  status: string;
  low_balance: boolean;
  valid_until?: string;
}

interface LedgerEntry {
  id: string;
  type: string;
  delta_hours: number;
  balance_after: number;
  reason: string;
  operator_name: string;
  occurred_at: string;
}

const items = ref<LessonAccount[]>([]);
const loading = ref(true);
const error = ref("");
const threshold = ref(2);
const selected = ref<LessonAccount | null>(null);
const ledger = ref<LedgerEntry[]>([]);
const hours = ref("");
const reason = ref("");
const saving = ref(false);

const typeLabel: Record<string, string> = {
  purchase: "充值",
  consume: "消课",
  adjust: "调整",
  refund: "退课",
};

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    const res = await request<{ items: LessonAccount[]; threshold: number }>("/api/v1/lesson-accounts", "GET");
    items.value = res.items;
    threshold.value = res.threshold;
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : "加载失败";
  } finally {
    loading.value = false;
  }
}

async function open(account: LessonAccount): Promise<void> {
  selected.value = account;
  hours.value = "";
  reason.value = "";
  ledger.value = [];
  try {
    const res = await request<{ account: LessonAccount; ledger: LedgerEntry[] }>(
      `/api/v1/lesson-accounts/${account.student_id}`,
      "GET",
    );
    ledger.value = res.ledger;
    selected.value = res.account;
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : "加载流水失败", icon: "none" });
  }
}

function close(): void {
  selected.value = null;
  ledger.value = [];
}

async function purchase(): Promise<void> {
  if (!selected.value) return;
  const value = Number(hours.value);
  if (!Number.isFinite(value) || value <= 0) {
    uni.showToast({ title: "请填写充值课时", icon: "none" });
    return;
  }
  saving.value = true;
  try {
    await request(`/api/v1/lesson-accounts/${selected.value.student_id}/purchase`, "POST", {
      hours: value,
      reason: reason.value.trim(),
    });
    uni.showToast({ title: "已充值", icon: "success" });
    await load();
    if (selected.value) {
      const target = items.value.find((item) => item.student_id === selected.value?.student_id);
      if (target) await open(target);
    }
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : "充值失败", icon: "none" });
  } finally {
    saving.value = false;
  }
}

async function adjust(): Promise<void> {
  if (!selected.value) return;
  const value = Number(hours.value);
  if (!Number.isFinite(value) || value === 0) {
    uni.showToast({ title: "调整课时不能为 0", icon: "none" });
    return;
  }
  if (!reason.value.trim()) {
    uni.showToast({ title: "调整必须填写原因", icon: "none" });
    return;
  }
  saving.value = true;
  try {
    await request(`/api/v1/lesson-accounts/${selected.value.student_id}/adjust`, "POST", {
      delta_hours: value,
      reason: reason.value.trim(),
    });
    uni.showToast({ title: "已调整", icon: "success" });
    await load();
    if (selected.value) {
      const target = items.value.find((item) => item.student_id === selected.value?.student_id);
      if (target) await open(target);
    }
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : "调整失败", icon: "none" });
  } finally {
    saving.value = false;
  }
}

function fmt(iso: string): string {
  const d = new Date(iso);
  return `${d.getMonth() + 1}月${d.getDate()}日 ${`${d.getHours()}`.padStart(2, "0")}:${`${d.getMinutes()}`.padStart(2, "0")}`;
}

onShow(load);
</script>

<template>
  <view class="page">
    <view class="head">
      <text class="title">课时台账</text>
      <text class="sub">余额 ≤ {{ threshold }} 标红</text>
    </view>

    <view v-if="loading" class="hint">加载中…</view>
    <view v-else-if="error" class="hint error" @tap="load">{{ error }}，点此重试</view>
    <view v-else-if="items.length === 0" class="hint">还没有学员，先在学员档案里建档。</view>

    <view v-for="item in items" :key="item.student_id" class="card" @tap="open(item)">
      <view class="row">
        <text class="name">{{ item.student_name }}</text>
        <text class="balance" :class="{ low: item.low_balance }">{{ item.balance_hours.toFixed(2) }} 课时</text>
      </view>
      <text class="meta">{{ item.grade || "未填年级" }} · 已购 {{ item.purchased_hours.toFixed(2) }} · 已用 {{ item.used_hours.toFixed(2) }}</text>
      <text v-if="item.valid_until" class="meta">有效期至 {{ item.valid_until }}</text>
    </view>

    <view v-if="selected" class="sheet">
      <view class="sheet-head">
        <text class="sheet-title">{{ selected.student_name }} · {{ selected.balance_hours.toFixed(2) }} 课时</text>
        <text class="link" @tap="close">关闭</text>
      </view>
      <input v-model="hours" class="input" type="digit" placeholder="课时数（调整可填负数，如 -1）" />
      <input v-model="reason" class="input" placeholder="原因（充值与调整都建议填写）" />
      <view class="sheet-actions">
        <button class="btn" :loading="saving" @tap="purchase">充值</button>
        <button class="btn ghost" :loading="saving" @tap="adjust">调整</button>
      </view>

      <text class="card-title">最近流水</text>
      <view v-if="ledger.length === 0" class="hint small">还没有流水</view>
      <view v-for="entry in ledger" :key="entry.id" class="ledger-row">
        <text class="ledger-type">{{ typeLabel[entry.type] ?? entry.type }}</text>
        <text class="ledger-delta" :class="{ minus: entry.delta_hours < 0 }">
          {{ entry.delta_hours > 0 ? "+" : "" }}{{ entry.delta_hours.toFixed(2) }}
        </text>
        <text class="ledger-meta">{{ fmt(entry.occurred_at) }} · 余 {{ entry.balance_after.toFixed(2) }}</text>
        <text v-if="entry.reason" class="ledger-reason">{{ entry.reason }}</text>
      </view>
    </view>
  </view>
</template>

<style lang="scss">
@import "@/styles/tokens.scss";
.page { min-height: 100vh; background: $hm-bg; padding: 24rpx 24rpx 60rpx; }
.head { display: flex; justify-content: space-between; align-items: baseline; margin-bottom: 20rpx; }
.title { font-size: 34rpx; font-weight: 700; }
.sub { color: $hm-muted; font-size: 23rpx; }
.card { background: $hm-surface; border: 1rpx solid $hm-rule; border-radius: $hm-radius; padding: 24rpx; margin-bottom: 16rpx; }
.row { display: flex; justify-content: space-between; align-items: center; }
.name { font-size: 29rpx; font-weight: 600; }
.balance { font-size: 26rpx; color: $hm-primary; }
.balance.low { color: $hm-danger; }
.meta { display: block; color: $hm-muted; font-size: 23rpx; margin-top: 6rpx; }
.sheet { position: fixed; left: 0; right: 0; bottom: 0; background: $hm-surface; border-top: 1rpx solid $hm-rule; border-radius: $hm-radius $hm-radius 0 0; padding: 26rpx 26rpx 40rpx; max-height: 86vh; overflow-y: auto; }
.sheet-head { display: flex; justify-content: space-between; align-items: center; }
.sheet-title { font-size: 28rpx; font-weight: 600; }
.link { color: $hm-primary; font-size: 25rpx; }
.input { margin-top: 18rpx; background: $hm-paper-deep; border-radius: 12rpx; padding: 18rpx 20rpx; font-size: 26rpx; }
.sheet-actions { display: flex; margin-top: 22rpx; }
.btn { background: $hm-primary; color: #fff; font-size: 26rpx; border-radius: 12rpx; margin-right: 16rpx; padding: 0 32rpx; }
.btn.ghost { background: transparent; color: $hm-primary; border: 1rpx solid $hm-primary; }
.card-title { display: block; font-size: 26rpx; font-weight: 600; margin-top: 30rpx; }
.ledger-row { border-top: 1rpx solid $hm-rule; padding: 16rpx 0; display: flex; flex-wrap: wrap; align-items: baseline; }
.ledger-type { font-size: 24rpx; width: 90rpx; }
.ledger-delta { font-size: 26rpx; color: $hm-primary; width: 110rpx; }
.ledger-delta.minus { color: $hm-danger; }
.ledger-meta { color: $hm-muted; font-size: 22rpx; }
.ledger-reason { width: 100%; color: #475569; font-size: 23rpx; margin-top: 6rpx; }
.hint { color: $hm-muted; text-align: center; padding: 40rpx 0; font-size: 25rpx; }
.hint.small { padding: 16rpx 0; font-size: 23rpx; }
.hint.error { color: $hm-danger; }
</style>
