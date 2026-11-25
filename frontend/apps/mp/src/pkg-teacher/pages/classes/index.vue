<script setup lang="ts">
import { computed, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";

import { ApiError, request } from "@/utils/request";

interface ClassItem {
  id: string;
  name: string;
  subject: string;
  start_date: string;
  end_date: string;
  schedule_note: string;
  status: string;
  member_count: number;
  session_count: number;
}

interface ClassMember {
  student_id: string;
  student_name: string;
  grade: string;
  group_code: string;
  joined_at: string;
  left_at: string;
}

interface AttendanceRow {
  student_id: string;
  student_name: string;
  status: string;
}

interface ClassSession {
  id: string;
  session_date: string;
  content: string;
  created_at: string;
  attendance: AttendanceRow[];
}

interface StudentItem {
  id: string;
  name: string;
  grade: string;
}

interface Detail {
  class: ClassItem;
  members: ClassMember[];
  sessions: ClassSession[];
}

const loading = ref(true);
const error = ref("");
const classes = ref<ClassItem[]>([]);
const students = ref<StudentItem[]>([]);

const detail = ref<Detail | null>(null);
const detailBusy = ref(false);
const detailError = ref("");

const showForm = ref(false);
const busy = ref(false);
const form = ref({ name: "", subject: "physics", start_date: "", end_date: "", schedule_note: "" });

const addBusy = ref(false);
const pickStudents = ref(false);
const pickedStudents = ref<string[]>([]);
const myPick = ref<string[]>([]);

const sessionForm = ref({ session_date: "", content: "", attendance: {} as Record<string, string> });
const sessionBusy = ref(false);
const showSession = ref(false);

const statusLabel: Record<string, string> = { active: "在读", archived: "已归档" };
const attendanceLabel: Record<string, string> = { present: "出勤", absent: "缺课", leave: "请假" };

const activeMembers = computed(() => (detail.value ? detail.value.members.filter((m) => !m.left_at) : []));

function shortDay(iso: string): string {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return `${date.getMonth() + 1}月${date.getDate()}日`;
}

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    const res = await request<{ items: ClassItem[] }>("/api/v1/classes", "GET");
    classes.value = res.items;
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : "读取班级失败";
  } finally {
    loading.value = false;
  }
}

async function loadStudents(): Promise<void> {
  if (students.value.length > 0) return;
  try {
    const res = await request<{ items: StudentItem[] }>("/api/v1/students", "GET");
    students.value = res.items;
  } catch {
    students.value = [];
  }
}

async function openDetail(item: ClassItem): Promise<void> {
  detailBusy.value = true;
  detailError.value = "";
  detail.value = null;
  try {
    detail.value = await request<Detail>(`/api/v1/classes/${item.id}`, "GET");
  } catch (err) {
    detailError.value = err instanceof ApiError ? err.message : "读取班级详情失败";
  } finally {
    detailBusy.value = false;
  }
}

function closeDetail(): void {
  detail.value = null;
  detailError.value = "";
  showSession.value = false;
  pickStudents.value = false;
}

async function createClass(): Promise<void> {
  if (busy.value) return;
  const name = form.value.name.trim();
  if (!name) {
    uni.showToast({ title: "请填写班级名称", icon: "none" });
    return;
  }
  busy.value = true;
  try {
    await request<{ class: ClassItem }>("/api/v1/classes", "POST", {
      name,
      subject: form.value.subject.trim() || "physics",
      start_date: form.value.start_date || null,
      end_date: form.value.end_date || null,
      schedule_note: form.value.schedule_note.trim(),
    });
    uni.showToast({ title: "已创建", icon: "success" });
    showForm.value = false;
    form.value = { name: "", subject: "physics", start_date: "", end_date: "", schedule_note: "" };
    await load();
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "创建失败", icon: "none" });
  } finally {
    busy.value = false;
  }
}

function openPick(): void {
  void loadStudents();
  myPick.value = activeMembers.value.map((m) => m.student_id);
  pickedStudents.value = [];
  pickStudents.value = true;
}

function togglePick(id: string): void {
  if (myPick.value.includes(id)) return;
  const index = pickedStudents.value.indexOf(id);
  if (index >= 0) pickedStudents.value.splice(index, 1);
  else pickedStudents.value.push(id);
}

async function confirmPick(): Promise<void> {
  if (!detail.value || pickedStudents.value.length === 0) {
    pickStudents.value = false;
    return;
  }
  addBusy.value = true;
  try {
    const res = await request<{ added: number }>(`/api/v1/classes/${detail.value.class.id}/members`, "POST", {
      student_ids: pickedStudents.value,
    });
    uni.showToast({ title: `已加入 ${res.added} 人`, icon: "none" });
    pickStudents.value = false;
    const current = detail.value.class;
    await openDetail(current);
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "加入失败", icon: "none" });
  } finally {
    addBusy.value = false;
  }
}

async function removeMember(studentID: string): Promise<void> {
  if (!detail.value) return;
  const confirmed = await new Promise<boolean>((resolve) => {
    uni.showModal({
      title: "移除成员",
      content: "只标记退出，历史出勤保留。",
      success: (res) => resolve(res.confirm),
      fail: () => resolve(false),
    });
  });
  if (!confirmed) return;
  try {
    await request<{ ok: boolean }>(`/api/v1/classes/${detail.value.class.id}/members/${studentID}`, "DELETE");
    const current = detail.value.class;
    await openDetail(current);
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "移除失败", icon: "none" });
  }
}

function openSession(): void {
  sessionForm.value = { session_date: "", content: "", attendance: {} };
  showSession.value = true;
}

function cycleAttendance(studentID: string): void {
  const order = ["present", "absent", "leave"];
  const current = sessionForm.value.attendance[studentID] ?? "present";
  const next = order[(order.indexOf(current) + 1) % order.length];
  sessionForm.value.attendance = { ...sessionForm.value.attendance, [studentID]: next };
}

async function createSession(): Promise<void> {
  if (!detail.value || sessionBusy.value) return;
  if (!sessionForm.value.session_date) {
    uni.showToast({ title: "请选择上课日期", icon: "none" });
    return;
  }
  sessionBusy.value = true;
  try {
    await request<{ session: ClassSession }>(`/api/v1/classes/${detail.value.class.id}/sessions`, "POST", {
      session_date: `${sessionForm.value.session_date}T00:00:00Z`,
      content: sessionForm.value.content.trim(),
      attendance: sessionForm.value.attendance,
    });
    uni.showToast({ title: "已记录", icon: "success" });
    showSession.value = false;
    const current = detail.value.class;
    await openDetail(current);
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "记录失败", icon: "none" });
  } finally {
    sessionBusy.value = false;
  }
}

async function deleteSession(sessionID: string): Promise<void> {
  if (!detail.value) return;
  try {
    await request<{ ok: boolean }>(`/api/v1/classes/${detail.value.class.id}/sessions/${sessionID}`, "DELETE");
    const current = detail.value.class;
    await openDetail(current);
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "删除失败", icon: "none" });
  }
}

async function archiveClass(item: ClassItem): Promise<void> {
  const confirmed = await new Promise<boolean>((resolve) => {
    uni.showModal({
      title: "归档班级",
      content: "归档后不再出现在默认列表，历史出勤保留。",
      success: (res) => resolve(res.confirm),
      fail: () => resolve(false),
    });
  });
  if (!confirmed) return;
  try {
    await request<{ class: ClassItem }>(`/api/v1/classes/${item.id}/archive`, "POST", {});
    uni.showToast({ title: "已归档", icon: "none" });
    await load();
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "归档失败", icon: "none" });
  }
}

onShow(() => {
  void load();
});
</script>

<template>
  <view class="page">
    <view class="head">
      <text class="title">班课</text>
      <text class="sub">不扣课时、不产生流水，只记出勤与要点</text>
    </view>

    <view class="toolbar">
      <view class="btn primary" hover-class="press" @tap="showForm = true">新建班级</view>
      <view class="btn" hover-class="press" @tap="load">刷新</view>
    </view>

    <view v-if="loading" class="hint">加载中…</view>
    <view v-else-if="error" class="hint" @tap="load">{{ error }}，点我重试</view>
    <view v-else-if="classes.length === 0" class="empty">还没有班级，先点「新建班级」。</view>

    <view v-for="item in classes" :key="item.id" class="card" hover-class="press" @tap="openDetail(item)">
      <view class="card-top">
        <text class="name">{{ item.name }}</text>
        <text class="badge">{{ statusLabel[item.status] ?? item.status }}</text>
      </view>
      <text class="meta">
        {{ item.subject }} · 成员 {{ item.member_count }} 人 · 课次 {{ item.session_count }}
      </text>
      <text class="meta">
        {{ shortDay(item.start_date) || "未填开始" }} 至 {{ shortDay(item.end_date) || "未填结束" }}
        <text v-if="item.schedule_note"> · {{ item.schedule_note }}</text>
      </text>
      <view class="card-actions">
        <text v-if="item.status === 'active'" class="link" @tap.stop="archiveClass(item)">归档</text>
      </view>
    </view>

    <!-- 新建班级表单 -->
    <view v-if="showForm" class="mask" @tap="showForm = false">
      <view class="sheet" @tap.stop>
        <text class="sheet-title">新建班级</text>
        <input v-model="form.name" class="input" placeholder="班级名称，如 高一物理 A 班" maxlength="128" />
        <input v-model="form.subject" class="input" placeholder="学科（默认 physics）" maxlength="32" />
        <picker mode="date" :value="form.start_date" @change="form.start_date = $event.detail.value">
          <view class="input">{{ form.start_date || "开始日期（可选）" }}</view>
        </picker>
        <picker mode="date" :value="form.end_date" @change="form.end_date = $event.detail.value">
          <view class="input">{{ form.end_date || "结束日期（可选）" }}</view>
        </picker>
        <input v-model="form.schedule_note" class="input" placeholder="上课节奏，如 每周六 9:00" maxlength="255" />
        <view class="btn primary" :class="{ disabled: busy }" @tap="createClass">{{ busy ? "保存中…" : "保存" }}</view>
      </view>
    </view>

    <!-- 班级详情 -->
    <view v-if="detailBusy || detail || detailError" class="mask" @tap="closeDetail">
      <view class="sheet tall" @tap.stop>
        <view v-if="detailBusy" class="hint">加载中…</view>
        <view v-else-if="detailError" class="hint">{{ detailError }}</view>
        <template v-else-if="detail">
          <view class="sheet-head">
            <text class="sheet-title">{{ detail.class.name }}</text>
            <text class="link" @tap="closeDetail">关闭</text>
          </view>

          <view class="row-actions">
            <view v-if="detail.class.status === 'active'" class="btn small" @tap="openPick">加入学员</view>
            <view v-if="detail.class.status === 'active'" class="btn small primary" @tap="openSession">记一次课</view>
          </view>

          <text class="section">成员 {{ activeMembers.length }}</text>
          <view v-if="activeMembers.length === 0" class="hint small">还没有成员</view>
          <view v-for="member in activeMembers" :key="member.student_id" class="line">
            <text class="line-main">{{ member.student_name }}</text>
            <text class="line-sub">{{ member.grade || member.group_code }}</text>
            <text class="link" @tap="removeMember(member.student_id)">移除</text>
          </view>

          <text class="section">上课记录 {{ detail.sessions.length }}</text>
          <view v-if="detail.sessions.length === 0" class="hint small">还没有记录</view>
          <view v-for="session in detail.sessions" :key="session.id" class="session">
            <view class="session-top">
              <text class="line-main">{{ shortDay(session.session_date) }}</text>
              <text class="link" @tap="deleteSession(session.id)">删除</text>
            </view>
            <text v-if="session.content" class="line-sub">{{ session.content }}</text>
            <view class="attendance">
              <text v-for="row in session.attendance" :key="row.student_id" class="att">
                {{ row.student_name }} · {{ attendanceLabel[row.status] ?? row.status }}
              </text>
            </view>
          </view>
        </template>
      </view>
    </view>

    <!-- 选择学员 -->
    <view v-if="pickStudents" class="mask" @tap="pickStudents = false">
      <view class="sheet tall" @tap.stop>
        <text class="sheet-title">加入学员</text>
        <text class="hint small">已在班级里的学员不可重复选择</text>
        <view v-for="student in students" :key="student.id" class="line" @tap="togglePick(student.id)">
          <text class="line-main" :class="{ dim: myPick.includes(student.id) }">{{ student.name }}</text>
          <text class="line-sub">{{ student.grade }}</text>
          <text class="check" :class="{ on: pickedStudents.includes(student.id) }">✓</text>
        </view>
        <view class="btn primary" :class="{ disabled: addBusy }" @tap="confirmPick">
          {{ addBusy ? "加入中…" : `加入 ${pickedStudents.length} 人` }}
        </view>
      </view>
    </view>

    <!-- 记一次课 -->
    <view v-if="showSession && detail" class="mask" @tap="showSession = false">
      <view class="sheet tall" @tap.stop>
        <text class="sheet-title">记一次课</text>
        <picker mode="date" :value="sessionForm.session_date" @change="sessionForm.session_date = $event.detail.value">
          <view class="input">{{ sessionForm.session_date || "上课日期" }}</view>
        </picker>
        <textarea v-model="sessionForm.content" class="textarea" placeholder="本次内容要点（可选）" maxlength="1000" />
        <text class="section">出勤（点一下切换）</text>
        <view v-for="member in activeMembers" :key="member.student_id" class="line" @tap="cycleAttendance(member.student_id)">
          <text class="line-main">{{ member.student_name }}</text>
          <text class="att">{{ attendanceLabel[sessionForm.attendance[member.student_id] ?? "present"] }}</text>
        </view>
        <view class="btn primary" :class="{ disabled: sessionBusy }" @tap="createSession">
          {{ sessionBusy ? "保存中…" : "保存记录" }}
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
  background: rgba(15, 118, 110, 0.1);
  color: $hm-primary;
  font-size: 22rpx;
}

.meta {
  display: block;
  margin-top: 8rpx;
  font-size: 24rpx;
  color: $hm-muted;
}

.card-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 10rpx;
}

.link {
  color: $hm-primary;
  font-size: 26rpx;
}

.hint,
.empty {
  padding: 60rpx 0;
  text-align: center;
  color: $hm-muted;
  font-size: 26rpx;
}

.hint.small {
  padding: 16rpx 0;
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

/* 小程序原生 input 有固定行高：显式 height + line-height，只给 padding 会裁掉文字；
   同名 class 也用于 picker 的 <view>，line-height 正好把内容垂直居中。 */
.input {
  height: 88rpx;
  line-height: 88rpx;
}

.textarea {
  height: 160rpx;
  padding: 18rpx 22rpx;
  line-height: 1.5;
}

.row-actions {
  display: flex;
  margin: 12rpx 0 6rpx;
}

.section {
  display: block;
  margin: 22rpx 0 10rpx;
  font-size: 26rpx;
  font-weight: 600;
  color: $hm-ink;
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

.line-main.dim {
  color: $hm-muted;
}

.line-sub {
  display: block;
  font-size: 24rpx;
  color: $hm-muted;
}

.check {
  width: 40rpx;
  text-align: right;
  color: transparent;
}

.check.on {
  color: $hm-primary;
}

.session {
  padding: 14rpx 0;
  border-bottom: 1rpx solid $hm-border;
}

.session-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.attendance {
  display: flex;
  flex-wrap: wrap;
  margin-top: 8rpx;
}

.att {
  margin-right: 18rpx;
  font-size: 24rpx;
  color: $hm-primary-strong;
}
</style>
