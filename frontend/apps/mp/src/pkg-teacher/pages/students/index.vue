<script setup lang="ts">
import { computed, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";
import { useSessionStore } from "@/stores/session";

interface Student {
  id: string;
  name: string;
  grade: string;
  school: string;
  group_code: string;
  status: string;
  tags: string[];
  next_follow_up_at?: string;
  note: string;
}

const groups = [
  { value: "", label: "全部" },
  { value: "class_active", label: "班课在读" },
  { value: "one_on_one", label: "一对一在读" },
  { value: "intent", label: "待转化" },
  { value: "visitor", label: "咨询访客" },
];
const groupLabels: Record<string, string> = {
  class_active: "班课在读",
  one_on_one: "一对一在读",
  intent: "待转化意向",
  visitor: "咨询访客",
};
const statusLabels: Record<string, string> = {
  active: "在读",
  graduated: "结课归档",
  lost: "流失",
};

const session = useSessionStore();
const students = ref<Student[]>([]);
const total = ref(0);
const query = ref("");
const group = ref("");
const loading = ref(false);
const errorText = ref("");
const creating = ref(false);
const saving = ref(false);
const selecting = ref(false);
const selected = ref<string[]>([]);
const batchBusy = ref(false);
const form = ref({
  name: "",
  grade: "高一",
  school: "",
  group_code: "visitor",
  status: "active",
  note: "",
});

const title = computed(() => (total.value ? `${total.value} 位学员` : "学员档案"));
const groupOptions = computed(() => groups.slice(1));

async function load(): Promise<void> {
  if (!session.isTeacher) {
    uni.reLaunch({ url: "/pkg-teacher/pages/login/index" });
    return;
  }
  loading.value = true;
  errorText.value = "";
  const params = ["page=1", "page_size=50"];
  if (query.value.trim()) params.push(`q=${encodeURIComponent(query.value.trim())}`);
  if (group.value) params.push(`group=${group.value}`);
  try {
    const res = await request<{ items: Student[]; total: number }>(
      `/api/v1/students?${params.join("&")}`,
      "GET",
    );
    students.value = res.items;
    total.value = res.total;
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      session.clear();
      uni.reLaunch({ url: "/pkg-teacher/pages/login/index" });
      return;
    }
    errorText.value = err instanceof ApiError ? err.message : "加载失败";
  } finally {
    loading.value = false;
  }
}

onShow(() => {
  void load();
});

function pickGroup(value: string): void {
  group.value = value;
  void load();
}

function onGroupChange(e: { detail: { value: number } }): void {
  form.value.group_code = groupOptions.value[e.detail.value]?.value ?? "visitor";
}

function openCreate(): void {
  creating.value = true;
}

function closeCreate(): void {
  creating.value = false;
}

async function submitCreate(): Promise<void> {
  if (!form.value.name.trim()) {
    uni.showToast({ title: "请填写学员姓名", icon: "none" });
    return;
  }
  saving.value = true;
  try {
    await request("/api/v1/students", "POST", {
      name: form.value.name.trim(),
      grade: form.value.grade,
      school: form.value.school.trim(),
      group_code: form.value.group_code,
      status: form.value.status,
      note: form.value.note.trim(),
    });
    uni.showToast({ title: "已新增", icon: "success" });
    creating.value = false;
    form.value = { name: "", grade: "高一", school: "", group_code: "visitor", status: "active", note: "" };
    await load();
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "新增失败", icon: "none" });
  } finally {
    saving.value = false;
  }
}

function openStudent(id: string): void {
  uni.navigateTo({ url: `/pkg-teacher/pages/student/index?id=${id}` });
}

function enterSelect(id: string): void {
  selecting.value = true;
  toggleSelect(id);
}

function toggleSelect(id: string): void {
  const i = selected.value.indexOf(id);
  if (i >= 0) selected.value.splice(i, 1);
  else selected.value.push(id);
}

function cancelSelect(): void {
  selecting.value = false;
  selected.value = [];
}

async function batchArchive(): Promise<void> {
  if (!selected.value.length) return;
  const res = await uni.showModal({
    title: "批量结课归档",
    content: `将选中的 ${selected.value.length} 位学员改为“结课归档”？历史作业和课时不会删除。`,
    confirmText: "归档",
  });
  if (!res.confirm) return;
  batchBusy.value = true;
  let ok = 0;
  try {
    for (const id of [...selected.value]) {
      try {
        await request(`/api/v1/students/${id}`, "PATCH", { status: "graduated" });
        ok++;
      } catch {
        // 单个失败不阻断其余学员，最后统一提示。
      }
    }
    uni.showToast({ title: `已归档 ${ok} 位`, icon: "success" });
    cancelSelect();
    await load();
  } finally {
    batchBusy.value = false;
  }
}
</script>

<template>
  <view class="page">
    <view class="head">
      <text class="title">{{ title }}</text>
      <button class="add" @click="openCreate">新增</button>
    </view>

    <view class="search">
      <input
        v-model="query"
        class="input"
        placeholder="搜索姓名 / 学校 / 备注"
        confirm-type="search"
        @confirm="load"
      />
      <button class="search-btn" @click="load">搜索</button>
    </view>

    <scroll-view class="chips" scroll-x>
      <text
        v-for="item in groups"
        :key="item.value"
        class="chip"
        :class="{ active: group === item.value }"
        @click="pickGroup(item.value)"
      >{{ item.label }}</text>
    </scroll-view>

    <view v-if="errorText" class="error">{{ errorText }}</view>
    <view v-else-if="loading" class="hint">正在加载…</view>
    <view v-else-if="!students.length" class="hint">还没有学员，点右上角新增。</view>

    <view
      v-for="student in students"
      :key="student.id"
      class="card"
      :class="{ selected: selected.includes(student.id) }"
      @longpress.stop="enterSelect(student.id)"
      @click="selecting ? toggleSelect(student.id) : openStudent(student.id)"
    >
      <view v-if="selecting" class="check" :class="{ on: selected.includes(student.id) }">
        {{ selected.includes(student.id) ? "✓" : "" }}
      </view>
      <view class="card-top">
        <text class="name">{{ student.name }}</text>
        <text class="grade">{{ student.grade }}</text>
      </view>
      <text class="meta">{{ student.school || "未填学校" }} · {{ groupLabels[student.group_code] || student.group_code }}</text>
      <view class="card-bottom">
        <text class="status" :class="student.status">{{ statusLabels[student.status] || student.status }}</text>
        <text v-if="student.next_follow_up_at" class="follow">回访 {{ student.next_follow_up_at.slice(5, 10) }}</text>
      </view>
    </view>

    <view v-if="selecting" class="batchbar">
      <view class="batch-count">
        <text class="batch-num">{{ selected.length }}</text>
        <text class="batch-label">已选</text>
      </view>
      <button class="batch-cancel" @click="cancelSelect">取消</button>
      <button class="batch-ok" :loading="batchBusy" :disabled="!selected.length || batchBusy" @click="batchArchive">
        结课归档
      </button>
    </view>

    <view v-if="creating" class="mask" @click.self="closeCreate">
      <view class="sheet" @click.stop>
        <text class="sheet-title">新增学员</text>
        <input v-model="form.name" class="field" placeholder="学员姓名" />
        <input v-model="form.grade" class="field" placeholder="年级，如 高一" />
        <input v-model="form.school" class="field" placeholder="学校" />
        <view class="field-row">
          <text class="field-label">分组</text>
          <picker
            :range="groupOptions"
            range-key="label"
            @change="onGroupChange"
          >
            <text class="picker">{{ groupLabels[form.group_code] || "选择" }}</text>
          </picker>
        </view>
        <textarea v-model="form.note" class="textarea" placeholder="备注（可选）" />
        <button class="primary" :loading="saving" @click="submitCreate">保存</button>
        <button class="ghost" @click="closeCreate">取消</button>
      </view>
    </view>
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

.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.title {
  font-size: 38rpx;
  font-weight: 700;
}

.add {
  height: 64rpx;
  padding: 0 26rpx;
  line-height: 64rpx;
  border-radius: 32rpx;
  font-size: 26rpx;
  background: $hm-primary;
  color: #fff;
}

.search {
  display: flex;
  gap: 12rpx;
  margin-top: 24rpx;
}

.input {
  flex: 1;
  height: 80rpx;
  padding: 0 24rpx;
  border-radius: $hm-radius;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  font-size: 28rpx;
}

.search-btn {
  width: 120rpx;
  height: 80rpx;
  line-height: 80rpx;
  border-radius: $hm-radius;
  font-size: 26rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

.chips {
  margin-top: 20rpx;
  white-space: nowrap;
}

.chip {
  display: inline-block;
  margin-right: 12rpx;
  padding: 12rpx 24rpx;
  border-radius: 30rpx;
  font-size: 24rpx;
  color: $hm-muted;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

.chip.active {
  color: #fff;
  background: $hm-primary;
  border-color: $hm-primary;
}

.hint,
.error {
  margin-top: 28rpx;
  font-size: 26rpx;
  color: $hm-muted;
}

.error {
  color: #b91c1c;
}

.card {
  position: relative;
  margin-top: 20rpx;
  padding: 24rpx;
  border-radius: $hm-radius;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

.card.selected {
  border-color: $hm-primary;
  background: #eef7f2;
}

.check {
  position: absolute;
  top: 18rpx;
  right: 18rpx;
  width: 40rpx;
  height: 40rpx;
  line-height: 40rpx;
  text-align: center;
  border-radius: 50%;
  border: 1rpx solid $hm-border;
  background: $hm-surface;
  font-size: 24rpx;
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
  gap: 16rpx;
  padding: 20rpx 32rpx calc(20rpx + env(safe-area-inset-bottom));
  background: $hm-surface;
  border-top: 1rpx solid $hm-border;
  z-index: 20;
}

.batch-count {
  display: flex;
  align-items: baseline;
  gap: 6rpx;
  margin-right: auto;
}

.batch-num {
  font-size: 40rpx;
  font-weight: 700;
  color: $hm-primary-strong;
}

.batch-label {
  font-size: 24rpx;
  color: $hm-muted;
}

.batch-cancel,
.batch-ok {
  height: 72rpx;
  padding: 0 28rpx;
  line-height: 72rpx;
  border-radius: 8rpx;
  font-size: 26rpx;
}

.batch-cancel {
  background: $hm-bg;
  color: $hm-ink;
}

.batch-ok {
  background: $hm-primary;
  color: #fff;
}

.card-top {
  display: flex;
  align-items: baseline;
  gap: 12rpx;
}

.name {
  font-size: 32rpx;
  font-weight: 600;
}

.grade {
  font-size: 24rpx;
  color: $hm-muted;
}

.meta {
  display: block;
  margin-top: 8rpx;
  font-size: 24rpx;
  color: $hm-muted;
}

.card-bottom {
  display: flex;
  justify-content: space-between;
  margin-top: 16rpx;
}

.status {
  font-size: 22rpx;
  padding: 4rpx 14rpx;
  border-radius: 20rpx;
  color: $hm-primary-strong;
  background: #ccfbf1;
}

.status.graduated,
.status.lost {
  color: #64748b;
  background: #e2e8f0;
}

.follow {
  font-size: 22rpx;
  color: $hm-muted;
}

.mask {
  position: fixed;
  inset: 0;
  z-index: 10;
  display: flex;
  align-items: flex-end;
  background: rgba(15, 23, 42, 0.45);
}

.sheet {
  width: 100%;
  max-height: 88vh;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
  padding: 30rpx $hm-space calc(30rpx + env(safe-area-inset-bottom));
  box-sizing: border-box;
  border-radius: 28rpx 28rpx 0 0;
  background: $hm-paper;
}

.sheet-title {
  display: block;
  margin-bottom: 24rpx;
  font-family: $hm-serif;
  font-size: 34rpx;
  font-weight: 700;
  color: $hm-ink;
}

.field,
.textarea {
  display: block;
  width: 100%;
  margin-top: 20rpx;
  box-sizing: border-box;
  border-radius: 10rpx;
  border: 1rpx solid $hm-rule;
  background: rgba(255, 255, 255, 0.55);
  color: $hm-ink;
  font-size: 28rpx;
}

.field {
  height: 88rpx;
  padding: 0 24rpx;
  line-height: 88rpx;
}

.textarea {
  height: 180rpx;
  padding: 20rpx 24rpx;
  line-height: 1.6;
}

.field-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 88rpx;
  margin-top: 20rpx;
  padding: 0 24rpx;
  box-sizing: border-box;
  border-radius: 10rpx;
  border: 1rpx solid $hm-rule;
  background: rgba(255, 255, 255, 0.55);
}

.field-label {
  font-size: 28rpx;
  color: rgba(34, 48, 42, 0.55);
}

.picker {
  font-size: 28rpx;
  color: $hm-primary-strong;
}

.primary,
.ghost {
  height: 88rpx;
  margin-top: 20rpx;
  line-height: 88rpx;
  border-radius: 10rpx;
  font-size: 28rpx;
}

.primary {
  background: $hm-primary-strong;
  color: #f7f3e8;
}

.ghost {
  background: transparent;
  color: $hm-primary-strong;
  border: 1rpx solid $hm-rule;
}
</style>
