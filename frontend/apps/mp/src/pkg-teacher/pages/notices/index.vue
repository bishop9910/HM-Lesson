<script setup lang="ts">
import { computed, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";

import { ApiError, request } from "@/utils/request";

interface NoticeItem {
  id: string;
  title: string;
  body: string;
  audience_type: string;
  audience_value: string;
  need_receipt: boolean;
  status: string;
  sent_at: string;
  created_at: string;
  receipt_total: number;
  receipt_done: number;
}

interface Receipt {
  id: string;
  guardian_name: string;
  phone: string;
  student_names: string;
  status: string;
  confirmed_at: string;
  reminded_at: string;
}

const loading = ref(true);
const error = ref("");
const notices = ref<NoticeItem[]>([]);

const showForm = ref(false);
const busy = ref(false);
const form = ref({ title: "", body: "", audience_type: "all", audience_value: "", need_receipt: true });
const audienceLabel: Record<string, string> = { all: "全部家长", group: "按分组", students: "指定学员" };
const groupOptions = [
  { value: "class_active", label: "班课在读" },
  { value: "one_on_one", label: "一对一在读" },
  { value: "intent", label: "待转化意向" },
  { value: "visitor", label: "咨询访客" },
];

const current = ref<NoticeItem | null>(null);
const receipts = ref<Receipt[]>([]);
const receiptsBusy = ref(false);
const pendingOnly = ref(false);
const remindBusy = ref(false);

const pendingCount = computed(() => receipts.value.filter((item) => item.status === "pending").length);

function shortTime(iso: string): string {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return `${date.getMonth() + 1}月${date.getDate()}日 ${String(date.getHours()).padStart(2, "0")}:${String(
    date.getMinutes(),
  ).padStart(2, "0")}`;
}

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    const res = await request<{ items: NoticeItem[] }>("/api/v1/notices", "GET");
    notices.value = res.items;
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : "读取通知失败";
  } finally {
    loading.value = false;
  }
}

async function createNotice(): Promise<void> {
  if (busy.value) return;
  const title = form.value.title.trim();
  const body = form.value.body.trim();
  if (!title || !body) {
    uni.showToast({ title: "标题和正文都要填", icon: "none" });
    return;
  }
  if (form.value.audience_type === "group" && !form.value.audience_value) {
    uni.showToast({ title: "请选择分组", icon: "none" });
    return;
  }
  busy.value = true;
  try {
    await request<{ notice: NoticeItem }>("/api/v1/notices", "POST", {
      title,
      body,
      audience_type: form.value.audience_type,
      audience_value: form.value.audience_value,
      need_receipt: form.value.need_receipt,
    });
    uni.showToast({ title: "已存草稿", icon: "success" });
    showForm.value = false;
    form.value = { title: "", body: "", audience_type: "all", audience_value: "", need_receipt: true };
    await load();
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "保存失败", icon: "none" });
  } finally {
    busy.value = false;
  }
}

async function send(item: NoticeItem): Promise<void> {
  const confirmed = await new Promise<boolean>((resolve) => {
    uni.showModal({
      title: "发送通知",
      content: "发送后不可修改，需要回执时会生成家长回执列表。",
      success: (res) => resolve(res.confirm),
      fail: () => resolve(false),
    });
  });
  if (!confirmed) return;
  try {
    const res = await request<{ notice: NoticeItem; receipts: number }>(`/api/v1/notices/${item.id}/send`, "POST", {});
    uni.showToast({ title: `已发送给 ${res.receipts} 位家长`, icon: "none" });
    await load();
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "发送失败", icon: "none" });
  }
}

async function openReceipts(item: NoticeItem): Promise<void> {
  current.value = item;
  receiptsBusy.value = true;
  pendingOnly.value = false;
  try {
    const res = await request<{ items: Receipt[] }>(`/api/v1/notices/${item.id}/receipts`, "GET");
    receipts.value = res.items;
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "读取回执失败", icon: "none" });
    receipts.value = [];
  } finally {
    receiptsBusy.value = false;
  }
}

function closeReceipts(): void {
  current.value = null;
  receipts.value = [];
}

async function remind(): Promise<void> {
  if (!current.value || remindBusy.value) return;
  remindBusy.value = true;
  try {
    const res = await request<{ ok: boolean; reminded: number }>(`/api/v1/notices/${current.value.id}/remind`, "POST", {});
    uni.showToast({ title: `已催办 ${res.reminded} 位家长`, icon: "none" });
    const notice = current.value;
    await openReceipts(notice);
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "催办失败", icon: "none" });
  } finally {
    remindBusy.value = false;
  }
}

async function deleteNotice(item: NoticeItem): Promise<void> {
  try {
    await request<{ ok: boolean }>(`/api/v1/notices/${item.id}`, "DELETE");
    uni.showToast({ title: "已删除", icon: "none" });
    await load();
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "删除失败", icon: "none" });
  }
}

const filteredReceipts = computed(() =>
  pendingOnly.value ? receipts.value.filter((item) => item.status === "pending") : receipts.value,
);

onShow(() => {
  void load();
});
</script>

<template>
  <view class="page">
    <view class="head">
      <text class="title">通知</text>
      <text class="sub">发送后按家长生成回执，48 小时未确认自动再推一次</text>
    </view>

    <view class="toolbar">
      <view class="btn primary" hover-class="press" @tap="showForm = true">新建通知</view>
      <view class="btn" hover-class="press" @tap="load">刷新</view>
    </view>

    <view v-if="loading" class="hint">加载中…</view>
    <view v-else-if="error" class="hint" @tap="load">{{ error }}，点我重试</view>
    <view v-else-if="notices.length === 0" class="empty">还没有通知。</view>

    <view v-for="item in notices" :key="item.id" class="card">
      <view class="card-top">
        <text class="name">{{ item.title }}</text>
        <text class="badge" :class="{ sent: item.status === 'sent' }">{{ item.status === "sent" ? "已发送" : "草稿" }}</text>
      </view>
      <text class="body">{{ item.body }}</text>
      <text class="meta">
        {{ audienceLabel[item.audience_type] ?? item.audience_type }}
        <text v-if="item.need_receipt"> · 需回执 {{ item.receipt_done }}/{{ item.receipt_total }}</text>
        <text v-if="item.sent_at"> · {{ shortTime(item.sent_at) }}</text>
      </text>
      <view class="actions">
        <text v-if="item.status === 'draft'" class="link" @tap="send(item)">发送</text>
        <text v-if="item.status === 'draft'" class="link danger" @tap="deleteNotice(item)">删除</text>
        <text v-if="item.status === 'sent'" class="link" @tap="openReceipts(item)">回执</text>
      </view>
    </view>

    <view v-if="showForm" class="mask" @tap="showForm = false">
      <view class="sheet" @tap.stop>
        <text class="sheet-title">新建通知</text>
        <input v-model="form.title" class="input" placeholder="标题，如 本周六家长会" maxlength="128" />
        <textarea v-model="form.body" class="textarea" placeholder="正文" maxlength="5000" />
        <view class="chips">
          <text
            v-for="(label, key) in audienceLabel"
            :key="key"
            class="chip"
            :class="{ on: form.audience_type === key }"
            @tap="form.audience_type = key"
          >
            {{ label }}
          </text>
        </view>
        <view v-if="form.audience_type === 'group'" class="chips">
          <text
            v-for="group in groupOptions"
            :key="group.value"
            class="chip"
            :class="{ on: form.audience_value === group.value }"
            @tap="form.audience_value = group.value"
          >
            {{ group.label }}
          </text>
        </view>
        <view class="switch-row" @tap="form.need_receipt = !form.need_receipt">
          <text class="switch-label">需要家长回执</text>
          <text class="switch" :class="{ on: form.need_receipt }">{{ form.need_receipt ? "开" : "关" }}</text>
        </view>
        <view class="btn primary" :class="{ disabled: busy }" @tap="createNotice">{{ busy ? "保存中…" : "存草稿" }}</view>
      </view>
    </view>

    <view v-if="current" class="mask" @tap="closeReceipts">
      <view class="sheet tall" @tap.stop>
        <view class="sheet-head">
          <text class="sheet-title">{{ current.title }}</text>
          <text class="link" @tap="closeReceipts">关闭</text>
        </view>
        <text class="hint small">已确认 {{ current.receipt_done }} / {{ current.receipt_total }} · 未确认 {{ pendingCount }}</text>
        <view class="row-actions">
          <view class="btn small" @tap="pendingOnly = !pendingOnly">{{ pendingOnly ? "看全部" : "只看未确认" }}</view>
          <view class="btn small primary" :class="{ disabled: remindBusy }" @tap="remind">
            {{ remindBusy ? "催办中…" : "一键催办" }}
          </view>
        </view>
        <view v-if="receiptsBusy" class="hint small">加载中…</view>
        <view v-else-if="filteredReceipts.length === 0" class="hint small">没有需要回执的家长</view>
        <view v-for="receipt in filteredReceipts" :key="receipt.id" class="line">
          <text class="line-main">{{ receipt.guardian_name || receipt.phone }}</text>
          <text class="line-sub">{{ receipt.student_names }}</text>
          <text class="status" :class="{ ok: receipt.status === 'confirmed' }">
            {{ receipt.status === "confirmed" ? "已确认" : "未确认" }}
          </text>
        </view>
      </view>
    </view>
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

.toolbar {
  display: flex;
  margin-bottom: 20rpx;
}

.btn {
  flex: 1;
  height: 76rpx;
  line-height: 76rpx;
  margin-right: 16rpx;
  border-radius: 16rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  color: $hm-ink;
  font-size: 28rpx;
  text-align: center;
}

.btn:last-child {
  margin-right: 0;
}

.btn.primary {
  background: $hm-primary;
  border-color: $hm-primary;
  color: #fff;
}

.btn.small {
  flex: none;
  padding: 0 26rpx;
  height: 64rpx;
  line-height: 64rpx;
  font-size: 26rpx;
}

.btn.disabled {
  opacity: 0.5;
}

.card {
  padding: 22rpx 24rpx;
  margin-bottom: 16rpx;
  border-radius: $hm-radius;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

.card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.name {
  font-size: 32rpx;
  font-weight: 600;
  color: $hm-ink;
}

.badge {
  padding: 2rpx 14rpx;
  border-radius: 999rpx;
  background: rgba(217, 119, 6, 0.12);
  color: $hm-accent;
  font-size: 22rpx;
}

.badge.sent {
  background: rgba(15, 118, 110, 0.1);
  color: $hm-primary;
}

.body {
  display: block;
  margin-top: 10rpx;
  font-size: 26rpx;
  color: $hm-ink;
  line-height: 1.5;
}

.meta {
  display: block;
  margin-top: 8rpx;
  font-size: 24rpx;
  color: $hm-muted;
}

.actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 10rpx;
}

.link {
  margin-left: 24rpx;
  color: $hm-primary;
  font-size: 26rpx;
}

.link.danger {
  color: #b91c1c;
}

.hint,
.empty {
  padding: 60rpx 0;
  text-align: center;
  color: $hm-muted;
  font-size: 26rpx;
}

.hint.small {
  padding: 12rpx 0;
  text-align: left;
}

.mask {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  background: rgba(0, 0, 0, 0.4);
  z-index: 20;
}

.sheet {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  max-height: 88vh;
  overflow-y: auto;
  padding: 24rpx;
  border-radius: $hm-radius $hm-radius 0 0;
  background: $hm-bg;
}

.sheet.tall {
  max-height: 92vh;
}

.sheet-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.sheet-title {
  display: block;
  margin-bottom: 14rpx;
  font-size: 32rpx;
  font-weight: 600;
  color: $hm-ink;
}

.input,
.textarea {
  width: 100%;
  box-sizing: border-box;
  margin-bottom: 14rpx;
  padding: 0 22rpx;
  border-radius: 14rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  color: $hm-ink;
  font-size: 28rpx;
}

/* 小程序原生 input 有固定行高，必须显式给 height + line-height，
   否则只加 padding 会把文字挤出可视区（上下被裁掉）。 */
.input {
  height: 88rpx;
  line-height: 88rpx;
}

.textarea {
  height: 220rpx;
  padding: 20rpx 22rpx;
  line-height: 1.5;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  margin-bottom: 12rpx;
}

.chip {
  margin: 0 12rpx 12rpx 0;
  padding: 10rpx 22rpx;
  border-radius: 999rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  color: $hm-muted;
  font-size: 24rpx;
}

.chip.on {
  background: rgba(15, 118, 110, 0.12);
  border-color: $hm-primary;
  color: $hm-primary;
}

.switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18rpx 0;
}

.switch-label {
  font-size: 28rpx;
  color: $hm-ink;
}

.switch {
  padding: 4rpx 22rpx;
  border-radius: 999rpx;
  border: 1rpx solid $hm-border;
  color: $hm-muted;
  font-size: 24rpx;
}

.switch.on {
  border-color: $hm-primary;
  color: $hm-primary;
}

.row-actions {
  display: flex;
  margin: 10rpx 0;
}

.line {
  display: flex;
  align-items: center;
  padding: 16rpx 0;
  border-bottom: 1rpx solid $hm-border;
}

.line-main {
  flex: 1;
  font-size: 28rpx;
  color: $hm-ink;
}

.line-sub {
  margin-right: 16rpx;
  font-size: 24rpx;
  color: $hm-muted;
}

.status {
  font-size: 24rpx;
  color: $hm-accent;
}

.status.ok {
  color: $hm-primary;
}
</style>
