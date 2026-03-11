<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from "vue";
import { createDiscreteApi, type DataTableColumns, NButton, NInput, NPopconfirm, NSpace, NSwitch, NTag } from "naive-ui";
import {
  createUser,
  listUserGroups,
  listUsers,
  resetUserPassword,
  updateUser,
  updateUserRole,
  updateUserStatus,
  type UserGroupItem,
  type UserItem,
} from "@/api/modules/admin";
import { useAuthStore } from "@/stores/auth";

const { message } = createDiscreteApi(["message"]);
const authStore = useAuthStore();

const loading = ref(false);
const rows = ref<UserItem[]>([]);
const groups = ref<UserGroupItem[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const keyword = ref("");
const roleFilter = ref("");
const statusFilter = ref("");

const showCreate = ref(false);
const showEdit = ref(false);
const showReset = ref(false);

const createForm = reactive({
  username: "",
  email: "",
  password: "",
  role: "user" as "user" | "admin" | "super_admin",
  group_id: 0,
  daily_limit: 0,
  concurrency_limit: 0,
  active: true,
});

const editForm = reactive({
  id: 0,
  username: "",
  email: "",
  group_id: 0,
  daily_limit: 0,
  concurrency_limit: 0,
});

const resetForm = reactive({
  id: 0,
  password: "",
});

const roleOptions = computed(() => {
  const base = [
    { label: "普通用户", value: "user" },
    { label: "管理员", value: "admin" },
  ];
  if (authStore.isSuperAdmin) {
    base.push({ label: "超级管理员", value: "super_admin" });
  }
  return base;
});

const groupOptions = computed(() => [
  { label: "不分组", value: 0 },
  ...groups.value.map((g) => ({ label: `${g.name}${g.is_default ? " (默认)" : ""}`, value: g.id })),
]);

const limitDraft = reactive({
  createDaily: "0",
  createConcurrency: "0",
  editDaily: "0",
  editConcurrency: "0",
  createDailyError: "",
  createConcurrencyError: "",
  editDailyError: "",
  editConcurrencyError: "",
});

function validateNonNegativeInteger(value: string, label: string): string {
  const trimmed = value.trim();
  if (!/^\d+$/.test(trimmed)) {
    return `请输入${label}（仅数字）`;
  }
  return "";
}

function syncCreateDraftFromForm() {
  limitDraft.createDaily = String(createForm.daily_limit);
  limitDraft.createConcurrency = String(createForm.concurrency_limit);
  limitDraft.createDailyError = "";
  limitDraft.createConcurrencyError = "";
}

function syncEditDraftFromForm() {
  limitDraft.editDaily = String(editForm.daily_limit);
  limitDraft.editConcurrency = String(editForm.concurrency_limit);
  limitDraft.editDailyError = "";
  limitDraft.editConcurrencyError = "";
}

function onCreateDailyLimitInput(value: string) {
  limitDraft.createDaily = value;
  const error = validateNonNegativeInteger(value, "每日次数");
  limitDraft.createDailyError = error;
  if (error) return;
  createForm.daily_limit = Number.parseInt(value.trim(), 10);
}

function onCreateConcurrencyLimitInput(value: string) {
  limitDraft.createConcurrency = value;
  const error = validateNonNegativeInteger(value, "并发上限");
  limitDraft.createConcurrencyError = error;
  if (error) return;
  createForm.concurrency_limit = Number.parseInt(value.trim(), 10);
}

function onEditDailyLimitInput(value: string) {
  limitDraft.editDaily = value;
  const error = validateNonNegativeInteger(value, "每日次数");
  limitDraft.editDailyError = error;
  if (error) return;
  editForm.daily_limit = Number.parseInt(value.trim(), 10);
}

function onEditConcurrencyLimitInput(value: string) {
  limitDraft.editConcurrency = value;
  const error = validateNonNegativeInteger(value, "并发上限");
  limitDraft.editConcurrencyError = error;
  if (error) return;
  editForm.concurrency_limit = Number.parseInt(value.trim(), 10);
}

function onCreateDailyLimitBlur() {
  if (limitDraft.createDailyError) {
    message.warning(limitDraft.createDailyError);
    return;
  }
  limitDraft.createDaily = String(createForm.daily_limit);
}

function onCreateConcurrencyLimitBlur() {
  if (limitDraft.createConcurrencyError) {
    message.warning(limitDraft.createConcurrencyError);
    return;
  }
  limitDraft.createConcurrency = String(createForm.concurrency_limit);
}

function onEditDailyLimitBlur() {
  if (limitDraft.editDailyError) {
    message.warning(limitDraft.editDailyError);
    return;
  }
  limitDraft.editDaily = String(editForm.daily_limit);
}

function onEditConcurrencyLimitBlur() {
  if (limitDraft.editConcurrencyError) {
    message.warning(limitDraft.editConcurrencyError);
    return;
  }
  limitDraft.editConcurrency = String(editForm.concurrency_limit);
}

function formatTime(raw: string | null): string {
  if (!raw) return "-";
  const d = new Date(raw);
  if (Number.isNaN(d.getTime())) return raw;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function roleTagType(role: UserItem["role"]) {
  if (role === "super_admin") return "error";
  if (role === "admin") return "warning";
  return "default";
}

function roleText(role: UserItem["role"]) {
  if (role === "super_admin") return "超级管理员";
  if (role === "admin") return "管理员";
  return "普通用户";
}

function statusTagType(status: UserItem["status"]) {
  return status === "active" ? "success" : "error";
}

const columns: DataTableColumns<UserItem> = [
  { title: "ID", key: "id", width: 72, align: "center" },
  { title: "用户名", key: "username", minWidth: 130 },
  { title: "邮箱", key: "email", minWidth: 180 },
  {
    title: "角色",
    key: "role",
    width: 110,
    align: "center",
    render: (row) =>
      h(NTag, { size: "small", type: roleTagType(row.role) }, { default: () => roleText(row.role) }),
  },
  {
    title: "状态",
    key: "status",
    width: 98,
    align: "center",
    render: (row) =>
      h(NTag, { size: "small", type: statusTagType(row.status) }, { default: () => (row.status === "active" ? "启用" : "禁用") }),
  },
  {
    title: "用户组",
    key: "group_name",
    minWidth: 120,
    render: (row) => row.group_name || "-",
  },
  {
    title: "配额(日/并发)",
    key: "limits",
    width: 140,
    align: "center",
    render: (row) => `${row.daily_limit || 0} / ${row.concurrency_limit || 0}`,
  },
  {
    title: "最近登录",
    key: "last_login_at",
    width: 168,
    align: "center",
    render: (row) => formatTime(row.last_login_at),
  },
  {
    title: "启用",
    key: "switch",
    width: 88,
    align: "center",
    render: (row) =>
      h(NSwitch, {
        value: row.status === "active",
        disabled: row.role === "super_admin" && !authStore.isSuperAdmin,
        onUpdateValue: async (value: boolean) => {
          await onToggleStatus(row, value);
        },
      }),
  },
  {
    title: "操作",
    key: "actions",
    width: 300,
    align: "center",
    render: (row) =>
      h(NSpace, { size: 6, wrapItem: false, justify: "center" }, {
        default: () => [
          h(NButton, { tertiary: true, size: "small", type: "info", onClick: () => openEdit(row) }, { default: () => "编辑" }),
          authStore.isSuperAdmin
            ? h(
                NButton,
                {
                  tertiary: true,
                  size: "small",
                  type: row.role === "super_admin" ? "warning" : "primary",
                  disabled: row.role === "super_admin",
                  onClick: () => onChangeRole(row, row.role === "admin" ? "user" : "admin"),
                },
                { default: () => (row.role === "admin" ? "降为用户" : "设为管理员") },
              )
            : null,
          authStore.isSuperAdmin && row.role !== "super_admin"
            ? h(
                NPopconfirm,
                { onPositiveClick: () => onChangeRole(row, "super_admin") },
                {
                  default: () => "确认转移超级管理员身份到该用户？",
                  trigger: () => h(NButton, { tertiary: true, size: "small", type: "error" }, { default: () => "设为超管" }),
                },
              )
            : null,
          h(NButton, { tertiary: true, size: "small", onClick: () => openReset(row) }, { default: () => "重置密码" }),
        ].filter(Boolean),
      }),
  },
];

async function loadUsers() {
  loading.value = true;
  try {
    const data = await listUsers({
      page: page.value,
      page_size: pageSize.value,
      keyword: keyword.value.trim() || undefined,
      role: roleFilter.value || undefined,
      status: statusFilter.value || undefined,
    });
    rows.value = data.items || [];
    total.value = data.total || 0;
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    loading.value = false;
  }
}

async function loadGroups() {
  try {
    groups.value = await listUserGroups();
  } catch {
    groups.value = [];
  }
}

async function onToggleStatus(row: UserItem, active: boolean) {
  try {
    await updateUserStatus(row.id, active);
    message.success("状态已更新");
    await loadUsers();
  } catch (error) {
    message.error((error as Error).message);
  }
}

async function onChangeRole(row: UserItem, role: "user" | "admin" | "super_admin") {
  try {
    await updateUserRole(row.id, role);
    message.success("角色已更新");
    await loadUsers();
  } catch (error) {
    message.error((error as Error).message);
  }
}

function openCreate() {
  syncCreateDraftFromForm();
  showCreate.value = true;
}

function openEdit(row: UserItem) {
  editForm.id = row.id;
  editForm.username = row.username;
  editForm.email = row.email;
  editForm.group_id = row.group_id || 0;
  editForm.daily_limit = row.daily_limit || 0;
  editForm.concurrency_limit = row.concurrency_limit || 0;
  syncEditDraftFromForm();
  showEdit.value = true;
}

function openReset(row: UserItem) {
  resetForm.id = row.id;
  resetForm.password = "";
  showReset.value = true;
}

async function onCreate() {
  if (!createForm.username.trim() || !createForm.email.trim() || createForm.password.length < 8) {
    message.warning("请填写完整信息，密码至少8位");
    return;
  }
  if (limitDraft.createDailyError || limitDraft.createConcurrencyError) {
    message.warning("请先修正数字输入项");
    return;
  }
  try {
    await createUser({
      username: createForm.username.trim(),
      email: createForm.email.trim(),
      password: createForm.password,
      role: createForm.role,
      group_id: createForm.group_id || undefined,
      daily_limit: Number(createForm.daily_limit) || 0,
      concurrency_limit: Number(createForm.concurrency_limit) || 0,
      status: createForm.active ? "active" : "disabled",
    });
    message.success("创建成功");
    showCreate.value = false;
    createForm.username = "";
    createForm.email = "";
    createForm.password = "";
    createForm.role = "user";
    createForm.group_id = 0;
    createForm.daily_limit = 0;
    createForm.concurrency_limit = 0;
    createForm.active = true;
    syncCreateDraftFromForm();
    await loadUsers();
  } catch (error) {
    message.error((error as Error).message);
  }
}

async function onSaveEdit() {
  if (limitDraft.editDailyError || limitDraft.editConcurrencyError) {
    message.warning("请先修正数字输入项");
    return;
  }
  try {
    await updateUser(editForm.id, {
      username: editForm.username.trim(),
      email: editForm.email.trim(),
      group_id: editForm.group_id || 0,
      daily_limit: Number(editForm.daily_limit) || 0,
      concurrency_limit: Number(editForm.concurrency_limit) || 0,
    });
    message.success("保存成功");
    showEdit.value = false;
    await loadUsers();
  } catch (error) {
    message.error((error as Error).message);
  }
}

async function onResetPassword() {
  if (resetForm.password.length < 8) {
    message.warning("密码至少 8 位");
    return;
  }
  try {
    await resetUserPassword(resetForm.id, resetForm.password);
    message.success("密码已重置");
    showReset.value = false;
  } catch (error) {
    message.error((error as Error).message);
  }
}

function onPageChange(next: number) {
  page.value = next;
  loadUsers();
}

onMounted(async () => {
  syncCreateDraftFromForm();
  await loadGroups();
  await loadUsers();
});
</script>

<template>
  <section>
    <header class="title-row">
      <div>
        <h2 class="section-title">用户管理</h2>
        <p class="section-subtitle">管理站点用户、角色和个人配额。</p>
      </div>
      <n-space>
        <n-button secondary @click="loadUsers">刷新</n-button>
        <n-button type="primary" @click="openCreate">新建用户</n-button>
      </n-space>
    </header>

    <n-card class="filter-card">
      <n-space :size="10" wrap>
        <n-input v-model:value="keyword" clearable placeholder="用户名/邮箱关键字" style="width: 220px" @keydown.enter="loadUsers" />
        <n-select v-model:value="roleFilter" clearable placeholder="角色筛选" :options="[{label:'普通用户',value:'user'},{label:'管理员',value:'admin'},{label:'超级管理员',value:'super_admin'}]" style="width: 150px" />
        <n-select v-model:value="statusFilter" clearable placeholder="状态筛选" :options="[{label:'启用',value:'active'},{label:'禁用',value:'disabled'}]" style="width: 140px" />
        <n-button type="primary" @click="loadUsers">查询</n-button>
      </n-space>
    </n-card>

    <n-card class="table-card">
      <n-data-table
        :columns="columns"
        :data="rows"
        :loading="loading"
        :bordered="false"
        :single-line="false"
        :pagination="{
          page,
          pageSize,
          itemCount: total,
          onUpdatePage: onPageChange
        }"
      />
    </n-card>

    <n-modal v-model:show="showCreate" preset="card" title="新建用户" style="max-width: 640px">
      <n-form label-placement="top">
        <n-grid :cols="24" :x-gap="12">
          <n-form-item-gi :span="12" label="用户名">
            <n-input v-model:value="createForm.username" />
          </n-form-item-gi>
          <n-form-item-gi :span="12" label="邮箱">
            <n-input v-model:value="createForm.email" />
          </n-form-item-gi>
          <n-form-item-gi :span="12" label="初始密码">
            <n-input v-model:value="createForm.password" type="password" show-password-on="click" />
          </n-form-item-gi>
          <n-form-item-gi :span="12" label="角色">
            <n-select v-model:value="createForm.role" :options="roleOptions" />
          </n-form-item-gi>
          <n-form-item-gi :span="12" label="用户组">
            <n-select v-model:value="createForm.group_id" :options="groupOptions" />
          </n-form-item-gi>
          <n-form-item-gi :span="6" label="每日次数">
            <n-input
              :value="limitDraft.createDaily"
              inputmode="numeric"
              placeholder="请输入每日次数"
              style="width: 100%"
              @update:value="onCreateDailyLimitInput"
              @blur="onCreateDailyLimitBlur"
            />
          </n-form-item-gi>
          <n-form-item-gi :span="6" label="并发上限">
            <n-input
              :value="limitDraft.createConcurrency"
              inputmode="numeric"
              placeholder="请输入并发上限"
              style="width: 100%"
              @update:value="onCreateConcurrencyLimitInput"
              @blur="onCreateConcurrencyLimitBlur"
            />
          </n-form-item-gi>
          <n-form-item-gi :span="12" label="启用状态">
            <n-switch v-model:value="createForm.active" />
          </n-form-item-gi>
        </n-grid>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showCreate = false">取消</n-button>
          <n-button type="primary" @click="onCreate">创建</n-button>
        </n-space>
      </template>
    </n-modal>

    <n-modal v-model:show="showEdit" preset="card" title="编辑用户" style="max-width: 640px">
      <n-form label-placement="top">
        <n-grid :cols="24" :x-gap="12">
          <n-form-item-gi :span="12" label="用户名">
            <n-input v-model:value="editForm.username" />
          </n-form-item-gi>
          <n-form-item-gi :span="12" label="邮箱">
            <n-input v-model:value="editForm.email" />
          </n-form-item-gi>
          <n-form-item-gi :span="12" label="用户组">
            <n-select v-model:value="editForm.group_id" :options="groupOptions" />
          </n-form-item-gi>
          <n-form-item-gi :span="6" label="每日次数">
            <n-input
              :value="limitDraft.editDaily"
              inputmode="numeric"
              placeholder="请输入每日次数"
              style="width: 100%"
              @update:value="onEditDailyLimitInput"
              @blur="onEditDailyLimitBlur"
            />
          </n-form-item-gi>
          <n-form-item-gi :span="6" label="并发上限">
            <n-input
              :value="limitDraft.editConcurrency"
              inputmode="numeric"
              placeholder="请输入并发上限"
              style="width: 100%"
              @update:value="onEditConcurrencyLimitInput"
              @blur="onEditConcurrencyLimitBlur"
            />
          </n-form-item-gi>
        </n-grid>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showEdit = false">取消</n-button>
          <n-button type="primary" @click="onSaveEdit">保存</n-button>
        </n-space>
      </template>
    </n-modal>

    <n-modal v-model:show="showReset" preset="card" title="重置密码" style="max-width: 420px">
      <n-form label-placement="top">
        <n-form-item label="新密码（至少8位）">
          <n-input v-model:value="resetForm.password" type="password" show-password-on="click" />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showReset = false">取消</n-button>
          <n-button type="primary" @click="onResetPassword">确认</n-button>
        </n-space>
      </template>
    </n-modal>
  </section>
</template>

<style scoped>
.title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 14px;
  margin-bottom: 14px;
}

.filter-card {
  border-radius: 14px;
  margin-bottom: 14px;
}

.table-card {
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.9);
  border: 1px solid rgba(20, 41, 78, 0.08);
}

@media (max-width: 760px) {
  .title-row {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
