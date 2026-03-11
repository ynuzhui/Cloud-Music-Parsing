<script setup lang="ts">
import { h, onMounted, reactive, ref } from "vue";
import { createDiscreteApi, type DataTableColumns, NButton, NPopconfirm, NSpace, NSwitch, NTag } from "naive-ui";
import { createUserGroup, deleteUserGroup, listUserGroups, updateUserGroup, type UserGroupItem } from "@/api/modules/admin";

const { message } = createDiscreteApi(["message"]);
const loading = ref(false);
const rows = ref<UserGroupItem[]>([]);
const showCreate = ref(false);
const showEdit = ref(false);

const form = reactive({
  name: "",
  description: "",
  daily_limit: 0,
  concurrency_limit: 0,
  unlimited_parse: false,
  is_default: false,
});

const editForm = reactive({
  id: 0,
  name: "",
  description: "",
  daily_limit: 0,
  concurrency_limit: 0,
  unlimited_parse: false,
  is_default: false,
});

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
  limitDraft.createDaily = String(form.daily_limit);
  limitDraft.createConcurrency = String(form.concurrency_limit);
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
  const error = validateNonNegativeInteger(value, "每日次数上限");
  limitDraft.createDailyError = error;
  if (error) return;
  form.daily_limit = Number.parseInt(value.trim(), 10);
}

function onCreateConcurrencyLimitInput(value: string) {
  limitDraft.createConcurrency = value;
  const error = validateNonNegativeInteger(value, "并发上限");
  limitDraft.createConcurrencyError = error;
  if (error) return;
  form.concurrency_limit = Number.parseInt(value.trim(), 10);
}

function onEditDailyLimitInput(value: string) {
  limitDraft.editDaily = value;
  const error = validateNonNegativeInteger(value, "每日次数上限");
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
  limitDraft.createDaily = String(form.daily_limit);
}

function onCreateConcurrencyLimitBlur() {
  if (limitDraft.createConcurrencyError) {
    message.warning(limitDraft.createConcurrencyError);
    return;
  }
  limitDraft.createConcurrency = String(form.concurrency_limit);
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

const columns: DataTableColumns<UserGroupItem> = [
  { title: "ID", key: "id", width: 72, align: "center" },
  { title: "组名", key: "name", minWidth: 140 },
  { title: "描述", key: "description", minWidth: 180, render: (row) => row.description || "-" },
  {
    title: "配额(日/并发)",
    key: "limits",
    width: 140,
    align: "center",
    render: (row) => (row.unlimited_parse ? "∞ / ∞" : `${row.daily_limit || 0} / ${row.concurrency_limit || 0}`),
  },
  {
    title: "无限解析",
    key: "unlimited_parse",
    width: 110,
    align: "center",
    render: (row) =>
      row.unlimited_parse ? h(NTag, { type: "success", size: "small" }, { default: () => "开启" }) : h("span", "关闭"),
  },
  { title: "成员数", key: "member_count", width: 90, align: "center" },
  {
    title: "默认组",
    key: "is_default",
    width: 100,
    align: "center",
    render: (row) =>
      h(NSwitch, {
        value: !!row.is_default,
        onUpdateValue: async (value: boolean) => {
          await onToggleDefault(row, value);
        },
      }),
  },
  {
    title: "状态",
    key: "tag",
    width: 90,
    align: "center",
    render: (row) =>
      row.is_default ? h(NTag, { type: "success", size: "small" }, { default: () => "默认" }) : h("span", "-"),
  },
  {
    title: "操作",
    key: "actions",
    width: 160,
    align: "center",
    render: (row) =>
      h(NSpace, { size: 6, wrapItem: false, justify: "center" }, {
        default: () => [
          h(NButton, { tertiary: true, size: "small", type: "info", onClick: () => openEdit(row) }, { default: () => "编辑" }),
          h(
            NPopconfirm,
            { onPositiveClick: () => onDelete(row) },
            {
              default: () => "确认删除该用户组？",
              trigger: () => h(NButton, { tertiary: true, size: "small", type: "error", disabled: row.is_default }, { default: () => "删除" }),
            },
          ),
        ],
      }),
  },
];

async function loadRows() {
  loading.value = true;
  try {
    rows.value = await listUserGroups();
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    loading.value = false;
  }
}

async function onCreate() {
  if (!form.unlimited_parse && (limitDraft.createDailyError || limitDraft.createConcurrencyError)) {
    message.warning("请先修正数字输入项");
    return;
  }
  if (!form.name.trim()) {
    message.warning("请填写组名");
    return;
  }
  try {
    await createUserGroup({
      name: form.name.trim(),
      description: form.description.trim(),
      daily_limit: Number(form.daily_limit) || 0,
      concurrency_limit: Number(form.concurrency_limit) || 0,
      unlimited_parse: !!form.unlimited_parse,
      is_default: !!form.is_default,
    });
    message.success("创建成功");
    showCreate.value = false;
    form.name = "";
    form.description = "";
    form.daily_limit = 0;
    form.concurrency_limit = 0;
    form.unlimited_parse = false;
    form.is_default = false;
    syncCreateDraftFromForm();
    await loadRows();
  } catch (error) {
    message.error((error as Error).message);
  }
}

function openCreate() {
  syncCreateDraftFromForm();
  showCreate.value = true;
}

function openEdit(row: UserGroupItem) {
  editForm.id = row.id;
  editForm.name = row.name;
  editForm.description = row.description || "";
  editForm.daily_limit = row.daily_limit || 0;
  editForm.concurrency_limit = row.concurrency_limit || 0;
  editForm.unlimited_parse = !!row.unlimited_parse;
  editForm.is_default = !!row.is_default;
  syncEditDraftFromForm();
  showEdit.value = true;
}

async function onSaveEdit() {
  if (!editForm.unlimited_parse && (limitDraft.editDailyError || limitDraft.editConcurrencyError)) {
    message.warning("请先修正数字输入项");
    return;
  }
  if (!editForm.name.trim()) {
    message.warning("组名不能为空");
    return;
  }
  try {
    await updateUserGroup(editForm.id, {
      name: editForm.name.trim(),
      description: editForm.description.trim(),
      daily_limit: Number(editForm.daily_limit) || 0,
      concurrency_limit: Number(editForm.concurrency_limit) || 0,
      unlimited_parse: !!editForm.unlimited_parse,
      is_default: !!editForm.is_default,
    });
    message.success("保存成功");
    showEdit.value = false;
    await loadRows();
  } catch (error) {
    message.error((error as Error).message);
  }
}

async function onToggleDefault(row: UserGroupItem, value: boolean) {
  if (!value) return;
  try {
    await updateUserGroup(row.id, { is_default: true });
    message.success("默认组已更新");
    await loadRows();
  } catch (error) {
    message.error((error as Error).message);
  }
}

async function onDelete(row: UserGroupItem) {
  try {
    await deleteUserGroup(row.id);
    message.success("删除成功");
    await loadRows();
  } catch (error) {
    message.error((error as Error).message);
  }
}

onMounted(() => {
  syncCreateDraftFromForm();
  loadRows();
});
</script>

<template>
  <section>
    <header class="title-row">
      <div>
        <h2 class="section-title">用户组管理</h2>
        <p class="section-subtitle">维护用户组与组级配额，默认组用于新用户自动归属。</p>
      </div>
      <n-space>
        <n-button secondary @click="loadRows">刷新</n-button>
        <n-button type="primary" @click="openCreate">新建用户组</n-button>
      </n-space>
    </header>

    <n-card class="table-card">
      <n-data-table :columns="columns" :data="rows" :loading="loading" :bordered="false" :single-line="false" />
    </n-card>

    <n-modal v-model:show="showCreate" preset="card" title="新建用户组" style="max-width: 620px">
      <n-form label-placement="top">
        <n-grid :cols="24" :x-gap="12">
          <n-form-item-gi :span="12" label="组名">
            <n-input v-model:value="form.name" />
          </n-form-item-gi>
          <n-form-item-gi :span="6" label="默认组">
            <n-switch v-model:value="form.is_default" />
          </n-form-item-gi>
          <n-form-item-gi :span="6" label="无限解析">
            <n-switch v-model:value="form.unlimited_parse" />
          </n-form-item-gi>
          <n-form-item-gi :span="24" label="描述">
            <n-input v-model:value="form.description" />
          </n-form-item-gi>
          <n-form-item-gi :span="12" label="每日次数上限">
            <n-input
              :value="limitDraft.createDaily"
              inputmode="numeric"
              placeholder="请输入每日次数上限"
              :disabled="form.unlimited_parse"
              style="width: 100%"
              @update:value="onCreateDailyLimitInput"
              @blur="onCreateDailyLimitBlur"
            />
          </n-form-item-gi>
          <n-form-item-gi :span="12" label="并发上限">
            <n-input
              :value="limitDraft.createConcurrency"
              inputmode="numeric"
              placeholder="请输入并发上限"
              :disabled="form.unlimited_parse"
              style="width: 100%"
              @update:value="onCreateConcurrencyLimitInput"
              @blur="onCreateConcurrencyLimitBlur"
            />
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

    <n-modal v-model:show="showEdit" preset="card" title="编辑用户组" style="max-width: 620px">
      <n-form label-placement="top">
        <n-grid :cols="24" :x-gap="12">
          <n-form-item-gi :span="12" label="组名">
            <n-input v-model:value="editForm.name" />
          </n-form-item-gi>
          <n-form-item-gi :span="6" label="默认组">
            <n-switch v-model:value="editForm.is_default" />
          </n-form-item-gi>
          <n-form-item-gi :span="6" label="无限解析">
            <n-switch v-model:value="editForm.unlimited_parse" />
          </n-form-item-gi>
          <n-form-item-gi :span="24" label="描述">
            <n-input v-model:value="editForm.description" />
          </n-form-item-gi>
          <n-form-item-gi :span="12" label="每日次数上限">
            <n-input
              :value="limitDraft.editDaily"
              inputmode="numeric"
              placeholder="请输入每日次数上限"
              :disabled="editForm.unlimited_parse"
              style="width: 100%"
              @update:value="onEditDailyLimitInput"
              @blur="onEditDailyLimitBlur"
            />
          </n-form-item-gi>
          <n-form-item-gi :span="12" label="并发上限">
            <n-input
              :value="limitDraft.editConcurrency"
              inputmode="numeric"
              placeholder="请输入并发上限"
              :disabled="editForm.unlimited_parse"
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
