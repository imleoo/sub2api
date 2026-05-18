<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <!-- Search -->
          <div class="flex-1 sm:max-w-64">
            <input
              v-model="searchQuery"
              type="text"
              placeholder="搜索模型ID..."
              class="input"
              @input="handleSearch"
            />
          </div>

          <!-- Provider filter -->
          <Select
            v-model="filterProvider"
            :options="providerOptions"
            class="w-36"
            @change="handleFilterChange"
          />

          <!-- Source filter -->
          <Select
            v-model="filterSource"
            :options="sourceOptions"
            class="w-32"
            @change="handleFilterChange"
          />

          <!-- Enabled filter -->
          <Select
            v-model="filterEnabled"
            :options="enabledOptions"
            class="w-28"
            @change="handleFilterChange"
          />

          <!-- Right: action buttons -->
          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <button
              class="btn btn-secondary"
              :disabled="syncing"
              :title="'手动同步远端价格数据'"
              @click="handleSync"
            >
              <Icon name="refresh" size="md" :class="syncing ? 'animate-spin' : ''" />
              <span class="ml-1 hidden sm:inline">{{ syncing ? '同步中...' : '手动同步' }}</span>
            </button>
            <button class="btn btn-primary" @click="openCreateModal">
              <Icon name="plus" size="md" class="mr-1" />
              添加自定义模型
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th class="min-w-48">模型ID</th>
                <th>提供商</th>
                <th>模式</th>
                <th class="text-right">上游输入价</th>
                <th class="text-right">上游输出价</th>
                <th class="text-right">自定义输入价</th>
                <th class="text-right">自定义输出价</th>
                <th class="text-right">折扣率</th>
                <th class="text-right">实际输入价</th>
                <th class="max-w-40">描述</th>
                <th class="text-center">启用</th>
                <th class="text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              <template v-if="loading">
                <tr>
                  <td colspan="12" class="py-12 text-center text-gray-500 dark:text-gray-400">
                    加载中...
                  </td>
                </tr>
              </template>
              <template v-else-if="items.length === 0">
                <tr>
                  <td colspan="12" class="py-12 text-center text-gray-500 dark:text-gray-400">
                    暂无数据
                  </td>
                </tr>
              </template>
              <template v-else>
                <tr
                  v-for="item in items"
                  :key="item.id"
                  class="hover:bg-gray-50 dark:hover:bg-dark-700/50"
                >
                  <!-- Model ID -->
                  <td>
                    <div class="flex items-center gap-2">
                      <span class="font-mono text-xs text-gray-800 dark:text-gray-200">{{ item.model_id }}</span>
                      <span
                        v-if="item.is_custom"
                        class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/40 dark:text-blue-300"
                      >
                        自定义
                      </span>
                      <span
                        v-else
                        class="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-gray-700 dark:text-gray-400"
                      >
                        同步
                      </span>
                    </div>
                    <div v-if="item.display_name" class="mt-0.5 text-xs text-gray-400">{{ item.display_name }}</div>
                  </td>

                  <!-- Provider -->
                  <td class="text-gray-600 dark:text-gray-400">{{ item.provider }}</td>

                  <!-- Mode -->
                  <td class="text-gray-600 dark:text-gray-400">{{ item.mode }}</td>

                  <!-- Upstream input price -->
                  <td class="text-right font-mono text-xs text-gray-600 dark:text-gray-400">
                    {{ formatTokenPrice(item.input_cost_per_token) }}
                  </td>

                  <!-- Upstream output price -->
                  <td class="text-right font-mono text-xs text-gray-600 dark:text-gray-400">
                    {{ formatTokenPrice(item.output_cost_per_token) }}
                  </td>

                  <!-- Custom input price -->
                  <td class="text-right font-mono text-xs text-gray-600 dark:text-gray-400">
                    {{ formatTokenPrice(item.custom_input_cost) }}
                  </td>

                  <!-- Custom output price -->
                  <td class="text-right font-mono text-xs text-gray-600 dark:text-gray-400">
                    {{ formatTokenPrice(item.custom_output_cost) }}
                  </td>

                  <!-- Discount rate -->
                  <td class="text-right font-mono text-xs text-gray-600 dark:text-gray-400">
                    {{ formatDiscountRate(item.discount_rate) }}
                  </td>

                  <!-- Effective input price (highlighted) -->
                  <td class="text-right">
                    <span class="font-mono text-xs font-semibold text-primary-600 dark:text-primary-400">
                      {{ formatEffectiveInputPrice(item) }}
                    </span>
                  </td>

                  <!-- Description -->
                  <td class="max-w-40 truncate text-xs text-gray-500 dark:text-gray-400" :title="item.description ?? undefined">
                    {{ item.description ? (item.description.length > 40 ? item.description.slice(0, 40) + '...' : item.description) : '—' }}
                  </td>

                  <!-- Enabled toggle -->
                  <td class="text-center">
                    <button
                      type="button"
                      role="switch"
                      :aria-checked="item.is_enabled"
                      :class="[
                        'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
                        item.is_enabled ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600'
                      ]"
                      :disabled="togglingId === item.id"
                      @click="handleToggleEnabled(item)"
                    >
                      <span
                        :class="[
                          'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                          item.is_enabled ? 'translate-x-4' : 'translate-x-0'
                        ]"
                      />
                    </button>
                  </td>

                  <!-- Actions -->
                  <td class="text-right">
                    <div class="flex items-center justify-end gap-1">
                      <button
                        type="button"
                        class="rounded p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-300"
                        title="编辑"
                        @click="openEditModal(item)"
                      >
                        <Icon name="edit" size="sm" />
                      </button>
                      <button
                        type="button"
                        :disabled="!item.is_custom"
                        :title="item.is_custom ? '删除' : '仅自定义模型可删除'"
                        :class="[
                          'rounded p-1.5',
                          item.is_custom
                            ? 'text-red-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20'
                            : 'cursor-not-allowed text-gray-200 dark:text-gray-700'
                        ]"
                        @click="item.is_custom && openDeleteConfirm(item)"
                      >
                        <Icon name="trash" size="sm" />
                      </button>
                    </div>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- Edit Modal -->
    <BaseDialog
      :show="showEditModal"
      :title="editingItem?.is_custom ? '编辑自定义模型' : '编辑模型定价'"
      width="wide"
      @close="closeEditModal"
    >
      <form id="edit-pricing-form" class="space-y-4" @submit.prevent="handleSave">
        <!-- Read-only fields for synced models -->
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">模型ID</label>
            <input
              :value="editForm.model_id"
              type="text"
              class="input bg-gray-50 dark:bg-dark-900"
              :disabled="!editingItem?.is_custom"
              readonly
            />
          </div>
          <div>
            <label class="input-label">显示名称</label>
            <input v-model="editForm.display_name" type="text" class="input" placeholder="可选" />
          </div>
        </div>

        <div v-if="editingItem?.is_custom" class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">提供商</label>
            <input v-model="editForm.provider" type="text" class="input" required />
          </div>
          <div>
            <label class="input-label">模式</label>
            <Select v-model="editForm.mode" :options="modeOptions" />
          </div>
        </div>

        <div>
          <label class="input-label">描述</label>
          <textarea v-model="editForm.description" rows="2" class="input" placeholder="可选"></textarea>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div v-if="editingItem?.is_custom">
            <label class="input-label">上游输入价（每 token）</label>
            <input
              v-model.number="editForm.input_cost_per_token"
              type="number"
              step="any"
              min="0"
              class="input"
              placeholder="例: 0.000003"
            />
          </div>
          <div v-if="editingItem?.is_custom">
            <label class="input-label">上游输出价（每 token）</label>
            <input
              v-model.number="editForm.output_cost_per_token"
              type="number"
              step="any"
              min="0"
              class="input"
              placeholder="例: 0.000015"
            />
          </div>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">自定义输入价（每 token）</label>
            <input
              v-model.number="editForm.custom_input_cost"
              type="number"
              step="any"
              min="0"
              class="input"
              placeholder="留空则使用上游价格"
            />
          </div>
          <div>
            <label class="input-label">自定义输出价（每 token）</label>
            <input
              v-model.number="editForm.custom_output_cost"
              type="number"
              step="any"
              min="0"
              class="input"
              placeholder="留空则使用上游价格"
            />
          </div>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">折扣率（0~1，留空无折扣）</label>
            <input
              v-model.number="editForm.discount_rate"
              type="number"
              step="0.01"
              min="0"
              max="1"
              class="input"
              placeholder="例: 0.9"
            />
          </div>
          <div class="flex items-center gap-3 pt-6">
            <label class="relative inline-flex cursor-pointer items-center">
              <input v-model="editForm.is_enabled" type="checkbox" class="sr-only peer" />
              <div
                :class="[
                  'peer relative h-5 w-9 rounded-full border-2 border-transparent transition-colors duration-200',
                  editForm.is_enabled ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600'
                ]"
              >
                <span
                  :class="[
                    'absolute top-0 inline-block h-4 w-4 transform rounded-full bg-white shadow transition duration-200',
                    editForm.is_enabled ? 'translate-x-4' : 'translate-x-0'
                  ]"
                />
              </div>
            </label>
            <span class="text-sm text-gray-700 dark:text-gray-300">启用</span>
          </div>
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeEditModal">取消</button>
          <button type="submit" form="edit-pricing-form" :disabled="saving" class="btn btn-primary">
            {{ saving ? '保存中...' : '保存' }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Create Modal -->
    <BaseDialog :show="showCreateModal" title="添加自定义模型" width="wide" @close="closeCreateModal">
      <form id="create-pricing-form" class="space-y-4" @submit.prevent="handleCreate">
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">模型ID <span class="text-red-500">*</span></label>
            <input v-model="createForm.model_id" type="text" class="input" required placeholder="例: my-custom-model" />
          </div>
          <div>
            <label class="input-label">显示名称</label>
            <input v-model="createForm.display_name" type="text" class="input" placeholder="可选" />
          </div>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">提供商 <span class="text-red-500">*</span></label>
            <input v-model="createForm.provider" type="text" class="input" required placeholder="例: custom" />
          </div>
          <div>
            <label class="input-label">模式 <span class="text-red-500">*</span></label>
            <Select v-model="createForm.mode" :options="modeOptions" />
          </div>
        </div>

        <div>
          <label class="input-label">描述</label>
          <textarea v-model="createForm.description" rows="2" class="input" placeholder="可选"></textarea>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">上游输入价（每 token）</label>
            <input
              v-model.number="createForm.input_cost_per_token"
              type="number"
              step="any"
              min="0"
              class="input"
              placeholder="例: 0.000003"
            />
          </div>
          <div>
            <label class="input-label">上游输出价（每 token）</label>
            <input
              v-model.number="createForm.output_cost_per_token"
              type="number"
              step="any"
              min="0"
              class="input"
              placeholder="例: 0.000015"
            />
          </div>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">缓存创建输入价（每 token）</label>
            <input
              v-model.number="createForm.cache_creation_input_token_cost"
              type="number"
              step="any"
              min="0"
              class="input"
              placeholder="可选"
            />
          </div>
          <div>
            <label class="input-label">缓存读取输入价（每 token）</label>
            <input
              v-model.number="createForm.cache_read_input_token_cost"
              type="number"
              step="any"
              min="0"
              class="input"
              placeholder="可选"
            />
          </div>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">自定义输入价（每 token）</label>
            <input
              v-model.number="createForm.custom_input_cost"
              type="number"
              step="any"
              min="0"
              class="input"
              placeholder="留空则使用上游价格"
            />
          </div>
          <div>
            <label class="input-label">自定义输出价（每 token）</label>
            <input
              v-model.number="createForm.custom_output_cost"
              type="number"
              step="any"
              min="0"
              class="input"
              placeholder="留空则使用上游价格"
            />
          </div>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">折扣率（0~1，留空无折扣）</label>
            <input
              v-model.number="createForm.discount_rate"
              type="number"
              step="0.01"
              min="0"
              max="1"
              class="input"
              placeholder="例: 0.9"
            />
          </div>
          <div class="flex items-center gap-3 pt-6">
            <label class="relative inline-flex cursor-pointer items-center">
              <input v-model="createForm.is_enabled" type="checkbox" class="sr-only peer" />
              <div
                :class="[
                  'peer relative h-5 w-9 rounded-full border-2 border-transparent transition-colors duration-200',
                  createForm.is_enabled ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600'
                ]"
              >
                <span
                  :class="[
                    'absolute top-0 inline-block h-4 w-4 transform rounded-full bg-white shadow transition duration-200',
                    createForm.is_enabled ? 'translate-x-4' : 'translate-x-0'
                  ]"
                />
              </div>
            </label>
            <span class="text-sm text-gray-700 dark:text-gray-300">启用</span>
          </div>
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeCreateModal">取消</button>
          <button type="submit" form="create-pricing-form" :disabled="saving" class="btn btn-primary">
            {{ saving ? '创建中...' : '创建' }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Delete Confirmation -->
    <ConfirmDialog
      :show="showDeleteModal"
      title="删除自定义模型"
      :message="`确认删除模型 「${deletingItem?.model_id}」？此操作不可撤销。`"
      confirm-text="删除"
      cancel-text="取消"
      danger
      @confirm="confirmDelete"
      @cancel="showDeleteModal = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import {
  listModelPricings,
  createModelPricing,
  updateModelPricing,
  deleteModelPricing,
  triggerModelPricingSync,
  type DBModelPricing,
  type CreateModelPricingRequest,
} from '@/api/admin/modelPricings'

const appStore = useAppStore()

// ==================== State ====================

const loading = ref(false)
const saving = ref(false)
const syncing = ref(false)
const togglingId = ref<number | null>(null)

const items = ref<DBModelPricing[]>([])
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

const searchQuery = ref('')
const filterProvider = ref('')
const filterSource = ref('')
const filterEnabled = ref('')

let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null

// Modal state
const showEditModal = ref(false)
const showCreateModal = ref(false)
const showDeleteModal = ref(false)
const editingItem = ref<DBModelPricing | null>(null)
const deletingItem = ref<DBModelPricing | null>(null)

// ==================== Form state ====================

const editForm = reactive<{
  model_id: string
  display_name: string
  description: string
  provider: string
  mode: string
  input_cost_per_token: number | null
  output_cost_per_token: number | null
  custom_input_cost: number | null
  custom_output_cost: number | null
  discount_rate: number | null
  is_enabled: boolean
}>({
  model_id: '',
  display_name: '',
  description: '',
  provider: '',
  mode: 'chat',
  input_cost_per_token: null,
  output_cost_per_token: null,
  custom_input_cost: null,
  custom_output_cost: null,
  discount_rate: null,
  is_enabled: true,
})

const createForm = reactive<{
  model_id: string
  display_name: string
  description: string
  provider: string
  mode: string
  input_cost_per_token: number | null
  output_cost_per_token: number | null
  cache_creation_input_token_cost: number | null
  cache_read_input_token_cost: number | null
  custom_input_cost: number | null
  custom_output_cost: number | null
  discount_rate: number | null
  is_enabled: boolean
}>({
  model_id: '',
  display_name: '',
  description: '',
  provider: 'custom',
  mode: 'chat',
  input_cost_per_token: null,
  output_cost_per_token: null,
  cache_creation_input_token_cost: null,
  cache_read_input_token_cost: null,
  custom_input_cost: null,
  custom_output_cost: null,
  discount_rate: null,
  is_enabled: true,
})

// ==================== Options ====================

const providerOptions = [
  { value: '', label: '全部提供商' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'google', label: 'Google' },
  { value: 'custom', label: 'Custom' },
]

const sourceOptions = [
  { value: '', label: '全部来源' },
  { value: 'synced', label: '同步' },
  { value: 'custom', label: '自定义' },
]

const enabledOptions = [
  { value: '', label: '全部状态' },
  { value: 'true', label: '已启用' },
  { value: 'false', label: '已禁用' },
]

const modeOptions = [
  { value: 'chat', label: 'chat' },
  { value: 'image_generation', label: 'image_generation' },
  { value: 'video_generation', label: 'video_generation' },
]

// ==================== Formatters ====================

const formatTokenPrice = (price: number | null | undefined): string => {
  if (price === null || price === undefined) return '—'
  const perMillion = price * 1_000_000
  return `$${perMillion.toFixed(2)} /M tok`
}

const formatDiscountRate = (rate: number | null | undefined): string => {
  if (rate === null || rate === undefined) return '—'
  return `${rate} (${(rate * 100).toFixed(0)}%)`
}

const formatEffectiveInputPrice = (item: DBModelPricing): string => {
  const basePrice = item.custom_input_cost ?? item.input_cost_per_token
  if (basePrice === null || basePrice === undefined) return '—'
  const discount = item.discount_rate ?? 1
  return formatTokenPrice(basePrice * discount)
}

// ==================== Data loading ====================

const load = async () => {
  loading.value = true
  try {
    const params: Record<string, unknown> = {
      page: pagination.page,
      page_size: pagination.page_size,
    }
    if (searchQuery.value) params.q = searchQuery.value
    if (filterProvider.value) params.provider = filterProvider.value
    if (filterSource.value === 'custom') params.is_custom = true
    if (filterSource.value === 'synced') params.is_custom = false
    if (filterEnabled.value === 'true') params.is_enabled = true
    if (filterEnabled.value === 'false') params.is_enabled = false

    const { data } = await listModelPricings(params)
    items.value = data.items ?? []
    pagination.total = data.total
    pagination.page = data.page
    pagination.page_size = data.page_size
  } catch {
    appStore.showError('加载模型定价失败')
  } finally {
    loading.value = false
  }
}

// ==================== Filter handlers ====================

const handleSearch = () => {
  if (searchDebounceTimer) clearTimeout(searchDebounceTimer)
  searchDebounceTimer = setTimeout(() => {
    pagination.page = 1
    load()
  }, 300)
}

const handleFilterChange = () => {
  pagination.page = 1
  load()
}

const handlePageChange = (page: number) => {
  pagination.page = page
  load()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  load()
}

// ==================== Sync ====================

const handleSync = async () => {
  syncing.value = true
  try {
    await triggerModelPricingSync()
    appStore.showSuccess('同步触发成功，后台正在更新价格数据')
    await load()
  } catch {
    appStore.showError('同步失败')
  } finally {
    syncing.value = false
  }
}

// ==================== Toggle enabled ====================

const handleToggleEnabled = async (item: DBModelPricing) => {
  togglingId.value = item.id
  try {
    const { data } = await updateModelPricing(item.id, { is_enabled: !item.is_enabled })
    const idx = items.value.findIndex(i => i.id === item.id)
    if (idx !== -1) {
      items.value[idx] = data
    }
  } catch {
    appStore.showError('更新失败')
  } finally {
    togglingId.value = null
  }
}

// ==================== Edit modal ====================

const openEditModal = (item: DBModelPricing) => {
  editingItem.value = item
  editForm.model_id = item.model_id
  editForm.display_name = item.display_name ?? ''
  editForm.description = item.description ?? ''
  editForm.provider = item.provider
  editForm.mode = item.mode
  editForm.input_cost_per_token = item.input_cost_per_token
  editForm.output_cost_per_token = item.output_cost_per_token
  editForm.custom_input_cost = item.custom_input_cost
  editForm.custom_output_cost = item.custom_output_cost
  editForm.discount_rate = item.discount_rate
  editForm.is_enabled = item.is_enabled
  showEditModal.value = true
}

const closeEditModal = () => {
  showEditModal.value = false
  editingItem.value = null
}

const handleSave = async () => {
  if (!editingItem.value) return
  saving.value = true
  try {
    const payload: Partial<CreateModelPricingRequest> = {
      display_name: editForm.display_name || null,
      description: editForm.description || null,
      custom_input_cost: editForm.custom_input_cost ?? null,
      custom_output_cost: editForm.custom_output_cost ?? null,
      discount_rate: editForm.discount_rate ?? null,
      is_enabled: editForm.is_enabled,
    }
    if (editingItem.value.is_custom) {
      payload.provider = editForm.provider
      payload.mode = editForm.mode
      payload.input_cost_per_token = editForm.input_cost_per_token ?? null
      payload.output_cost_per_token = editForm.output_cost_per_token ?? null
    }
    const { data } = await updateModelPricing(editingItem.value.id, payload)
    const idx = items.value.findIndex(i => i.id === editingItem.value!.id)
    if (idx !== -1) items.value[idx] = data
    appStore.showSuccess('保存成功')
    closeEditModal()
  } catch {
    appStore.showError('保存失败')
  } finally {
    saving.value = false
  }
}

// ==================== Create modal ====================

const openCreateModal = () => {
  createForm.model_id = ''
  createForm.display_name = ''
  createForm.description = ''
  createForm.provider = 'custom'
  createForm.mode = 'chat'
  createForm.input_cost_per_token = null
  createForm.output_cost_per_token = null
  createForm.cache_creation_input_token_cost = null
  createForm.cache_read_input_token_cost = null
  createForm.custom_input_cost = null
  createForm.custom_output_cost = null
  createForm.discount_rate = null
  createForm.is_enabled = true
  showCreateModal.value = true
}

const closeCreateModal = () => {
  showCreateModal.value = false
}

const handleCreate = async () => {
  saving.value = true
  try {
    const payload: CreateModelPricingRequest = {
      model_id: createForm.model_id,
      display_name: createForm.display_name || null,
      description: createForm.description || null,
      provider: createForm.provider,
      mode: createForm.mode,
      input_cost_per_token: createForm.input_cost_per_token ?? null,
      output_cost_per_token: createForm.output_cost_per_token ?? null,
      cache_creation_input_token_cost: createForm.cache_creation_input_token_cost ?? null,
      cache_read_input_token_cost: createForm.cache_read_input_token_cost ?? null,
      custom_input_cost: createForm.custom_input_cost ?? null,
      custom_output_cost: createForm.custom_output_cost ?? null,
      discount_rate: createForm.discount_rate ?? null,
      is_enabled: createForm.is_enabled,
    }
    await createModelPricing(payload)
    appStore.showSuccess('创建成功')
    closeCreateModal()
    pagination.page = 1
    await load()
  } catch {
    appStore.showError('创建失败')
  } finally {
    saving.value = false
  }
}

// ==================== Delete ====================

const openDeleteConfirm = (item: DBModelPricing) => {
  deletingItem.value = item
  showDeleteModal.value = true
}

const confirmDelete = async () => {
  if (!deletingItem.value) return
  try {
    await deleteModelPricing(deletingItem.value.id)
    appStore.showSuccess('删除成功')
    showDeleteModal.value = false
    deletingItem.value = null
    await load()
  } catch {
    appStore.showError('删除失败')
  }
}

// ==================== Lifecycle ====================

onMounted(load)
</script>
