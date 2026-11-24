<script setup lang="ts">
import { onLoad } from "@dcloudio/uni-app";
import { ref } from "vue";
import { ApiError, request } from "@/utils/request";
import { useSessionStore } from "@/stores/session";

interface Staff {
  id: string;
  name: string;
  phone: string;
  role: "owner" | "assistant";
  status: "active" | "disabled";
}

const session = useSessionStore();
const list = ref<Staff[]>([]);
const loading = ref(false);
const name = ref("");
const phone = ref("");
const password = ref("");
const role = ref<"owner" | "assistant">("assistant");
const creating = ref(false);
const selecting = ref(false);
const picked = ref<string[]>([]);
const batchBusy = ref(false);

function toast(title: string): void {
  uni.showToast({ title, icon: "none" });
}

async function load(): Promise<void> {
  loading.value = true;
  try {
    const res = await request<{ items: Staff[] }>("/api/v1/staff", "GET");
    list.value = res.items;
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      session.clear();
      uni.reLaunch({ url: "/pkg-teacher/pages/login/index" });
      return;
    }
    toast(err instanceof ApiError ? err.message : "加载失败");
  } finally {
    loading.value = false;
  }
}

async function create(): Promise<void> {
  if (creating.value) return;
  if (!name.value.trim() || phone.value.trim().length < 6 || password.value.length < 8) {
    toast("姓名、手机号必填，密码至少 8 位");
    return;
  }
  creating.value = true;
  try {
    await request("/api/v1/staff", "POST", {
      name: name.value.trim(),
      phone: phone.value.trim(),
      password: password.value,
      role: role.value,
    });
    name.value = "";
    phone.value = "";
    password.value = "";
    toast("已添加");
    await load();
  } catch (err) {
    toast(err instanceof ApiError ? err.message : "添加失败");
  } finally {
    creating.value = false;
  }
}

async function toggle(item: Staff): Promise<void> {
  const next = item.status === "active" ? "disabled" : "active";
  try {
    await request(`/api/v1/staff/${item.id}`, "PATCH", { status: next });
    await load();
  } catch (err) {
    toast(err instanceof ApiError ? err.message : "操作失败");
  }
}

function back(): void {
  uni.navigateBack();
}

function enterSelect(id: string): void {
  selecting.value = true;
  togglePick(id);
}

function togglePick(id: string): void {
  const i = picked.value.indexOf(id);
  if (i >= 0) picked.value.splice(i, 1);
  else picked.value.push(id);
}

function cancelSelect(): void {
  selecting.value = false;
  picked.value = [];
}

async function batchStatus(next: "active" | "disabled"): Promise<void> {
  const targets = list.value.filter((s) => picked.value.includes(s.id) && s.status !== next);
  if (!targets.length) {
    toast(next === "disabled" ? "选中的成员都已是停用状态" : "选中的成员都已是启用状态");
    return;
  }
  const res = await uni.showModal({
    title: next === "disabled" ? "批量停用" : "批量启用",
    content: `${next === "disabled" ? "停用" : "启用"}选中的 ${targets.length} 位成员？`,
  });
  if (!res.confirm) return;
  batchBusy.value = true;
  let ok = 0;
  try {
    for (const item of targets) {
      try {
        await request(`/api/v1/staff/${item.id}`, "PATCH", { status: next });
        ok++;
      } catch {
        // 自己/最后一个 owner 会被服务端拒绝，跳过继续
      }
    }
    toast(`已处理 ${ok} 位`);
    cancelSelect();
    await load();
  } finally {
    batchBusy.value = false;
  }
}

onLoad(() => {
  if (!session.isOwner) {
    toast("仅负责人可管理成员");
    uni.reLaunch({ url: "/pkg-teacher/pages/workbench/index" });
    return;
  }
  void load();
});
</script>

<template>
  <view class="page">
    <view class="header">
      <text class="title">成员管理</text>
      <text class="subtitle">负责人可添加老师；助教只能处理作业、预约与资料。</text>
    </view>

    <view class="card">
      <text class="section">新增老师</text>
      <input v-model="name" class="input" placeholder="姓名" maxlength="20" />
      <input v-model="phone" class="input" type="number" maxlength="11" placeholder="手机号" />
      <input v-model="password" class="input" password maxlength="64" placeholder="初始密码（至少 8 位）" />
      <view class="role-row">
        <button class="chip" :class="{ active: role === 'assistant' }" @click="role = 'assistant'">助教</button>
        <button class="chip" :class="{ active: role === 'owner' }" @click="role = 'owner'">负责人</button>
      </view>
      <button class="primary" :loading="creating" :disabled="creating" @click="create">
        {{ creating ? "添加中…" : "添加" }}
      </button>
    </view>

    <view class="list">
      <view
        v-for="item in list"
        :key="item.id"
        class="row"
        :class="{ selected: picked.includes(item.id) }"
        @longpress.stop="enterSelect(item.id)"
        @click="selecting ? togglePick(item.id) : undefined"
      >
        <view v-if="selecting" class="check" :class="{ on: picked.includes(item.id) }">
          {{ picked.includes(item.id) ? "✓" : "" }}
        </view>
        <view class="row-main">
          <text class="row-title">{{ item.name }} · {{ item.role === "owner" ? "负责人" : "助教" }}</text>
          <text class="row-hint">{{ item.phone }} · {{ item.status === "active" ? "启用" : "已停用" }}</text>
        </view>
        <button v-if="!selecting" class="link" @click="toggle(item)">{{ item.status === "active" ? "停用" : "启用" }}</button>
      </view>
      <text v-if="!loading && list.length === 0" class="empty">还没有成员</text>
    </view>

    <view v-if="selecting" class="batchbar">
      <text class="batch-num">{{ picked.length }} 已选</text>
      <button class="batch-cancel" @click="cancelSelect">取消</button>
      <button class="batch-off" :disabled="batchBusy" @click="batchStatus('disabled')">停用</button>
      <button class="batch-on" :disabled="batchBusy" @click="batchStatus('active')">启用</button>
    </view>

    <button class="ghost" @click="back">返回</button>
  </view>
</template>

<style lang="scss" scoped>
@import "@/styles/tokens.scss";

.page {
  min-height: 100vh;
  padding: 32rpx $hm-space calc(160rpx + env(safe-area-inset-bottom));
  box-sizing: border-box;
  background: $hm-bg;
}

.header {
  display: flex;
  flex-direction: column;
  gap: 8rpx;
  margin-bottom: 24rpx;
}

.title {
  font-size: 40rpx;
  font-weight: 700;
}

.subtitle {
  font-size: 24rpx;
  color: $hm-muted;
  line-height: 1.5;
}

.card {
  display: flex;
  flex-direction: column;
  gap: 12rpx;
  padding: 24rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  border-radius: $hm-radius;
}

.section {
  font-size: 28rpx;
  font-weight: 600;
}

.input {
  height: 80rpx;
  padding: 0 18rpx;
  background: $hm-bg;
  border: 1rpx solid $hm-border;
  border-radius: 14rpx;
}

.role-row {
  display: flex;
  gap: 12rpx;
}

.chip {
  flex: 1;
  height: 64rpx;
  line-height: 64rpx;
  font-size: 24rpx;
  border-radius: 32rpx;
  background: $hm-bg;
  color: $hm-muted;
  border: 1rpx solid $hm-border;
}

.chip.active {
  color: $hm-primary-strong;
  border-color: $hm-primary;
  background: #ccfbf1;
}

.primary,
.ghost {
  height: 80rpx;
  line-height: 80rpx;
  border-radius: 40rpx;
  font-size: 28rpx;
}

.primary {
  margin-top: 8rpx;
  background: $hm-primary;
  color: #ffffff;
}

.ghost {
  margin-top: 28rpx;
  background: $hm-surface;
  color: $hm-primary-strong;
  border: 1rpx solid $hm-primary;
}

.list {
  margin-top: 24rpx;
}

.row {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 22rpx 20rpx;
  margin-bottom: 12rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  border-radius: $hm-radius;
}

.row.selected {
  border-color: $hm-primary;
  background: #eef7f2;
}

.check {
  position: absolute;
  top: 12rpx;
  right: 12rpx;
  width: 36rpx;
  height: 36rpx;
  line-height: 36rpx;
  text-align: center;
  border-radius: 50%;
  border: 1rpx solid $hm-border;
  background: $hm-surface;
  font-size: 22rpx;
  color: #fff;
}

.check.on {
  border-color: $hm-primary;
  background: $hm-primary;
}

.batchbar {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  gap: 12rpx;
  padding: 20rpx 24rpx calc(20rpx + env(safe-area-inset-bottom));
  background: $hm-surface;
  border-top: 1rpx solid $hm-border;
  z-index: 20;
}

.batch-num {
  margin-right: auto;
  font-size: 26rpx;
  color: $hm-muted;
}

.batch-cancel,
.batch-off,
.batch-on {
  height: 72rpx;
  padding: 0 24rpx;
  line-height: 72rpx;
  border-radius: 8rpx;
  font-size: 25rpx;
}

.batch-cancel {
  background: $hm-bg;
  color: $hm-ink;
}

.batch-off {
  background: #b91c1c;
  color: #fff;
}

.batch-on {
  background: $hm-primary;
  color: #fff;
}

.row-main {
  display: flex;
  flex-direction: column;
  gap: 6rpx;
}

.row-title {
  font-size: 28rpx;
  font-weight: 600;
}

.row-hint {
  font-size: 22rpx;
  color: $hm-muted;
}

.link {
  padding: 0;
  margin: 0;
  background: transparent;
  color: $hm-primary-strong;
  font-size: 24rpx;
  line-height: 1;
}

.empty {
  display: block;
  padding: 24rpx 0;
  font-size: 24rpx;
  color: $hm-muted;
  text-align: center;
}
</style>
