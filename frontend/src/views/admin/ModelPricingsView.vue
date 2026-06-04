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

          <!-- User-visible filter -->
          <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
            <input
              v-model="visibleOnly"
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              @change="handleFilterChange"
            />
            <span>只显示可见模型</span>
          </label>

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
            <button
              class="btn btn-secondary"
              :disabled="clearingDiscounts"
              :title="'将全表所有折扣率清空（置为无折扣）'"
              @click="showClearDiscountsModal = true"
            >
              <Icon name="x" size="md" />
              <span class="ml-1 hidden sm:inline">清空折扣</span>
            </button>
            <button
              class="btn btn-secondary"
              :title="'从上游渠道 /v1/models 拉取模型并入库'"
              @click="openImportModal"
            >
              <Icon name="download" size="md" />
              <span class="ml-1 hidden sm:inline">从上游导入</span>
            </button>
            <button
              class="btn btn-secondary"
              :disabled="wanjieSyncing"
              :title="'从万界 MaaS 平台同步定价数据'"
              @click="openWanjieModal"
            >
              <Icon name="refresh" size="md" :class="wanjieSyncing ? 'animate-spin' : ''" />
              <span class="ml-1 hidden sm:inline">{{ wanjieSyncing ? '同步中...' : '同步万界' }}</span>
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
                    {{ formatUpstreamInputPrice(item) }}
                  </td>

                  <!-- Upstream output price -->
                  <td class="text-right font-mono text-xs text-gray-600 dark:text-gray-400">
                    {{ formatUpstreamOutputPrice(item) }}
                  </td>

                  <!-- Custom input price -->
                  <td class="text-right font-mono text-xs text-gray-600 dark:text-gray-400">
                    {{ item.pricing_unit === 'second' ? formatSecondPrice(item.custom_input_cost) : formatTokenPrice(item.custom_input_cost) }}
                  </td>

                  <!-- Custom output price -->
                  <td class="text-right font-mono text-xs text-gray-600 dark:text-gray-400">
                    {{ item.pricing_unit === 'second' ? '—' : formatTokenPrice(item.custom_output_cost) }}
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

        <div>
          <label class="input-label">{{ t('admin.modelPricings.pricingUnitLabel') }}</label>
          <div class="mt-1 flex overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
            <label
              v-for="option in pricingUnitOptions"
              :key="option.value"
              class="flex-1 cursor-pointer px-3 py-2 text-center text-sm transition-colors"
              :class="
                editForm.pricing_unit === option.value
                  ? 'bg-primary-600 text-white'
                  : 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700'
              "
            >
              <input v-model="editForm.pricing_unit" type="radio" class="sr-only" :value="option.value" />
              {{ option.label }}
            </label>
          </div>
        </div>

        <div v-if="editForm.pricing_unit === 'token'" class="grid grid-cols-1 gap-4 md:grid-cols-2">
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

        <div v-else class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div v-if="editingItem?.is_custom">
            <label class="input-label">{{ t('admin.modelPricings.upstreamPricePerSecond') }}</label>
            <input
              v-model.number="editForm.input_cost_per_token"
              type="number"
              step="any"
              min="0"
              class="input"
              placeholder="例: 0.12"
            />
          </div>
        </div>

        <div v-if="editForm.pricing_unit === 'token'" class="grid grid-cols-1 gap-4 md:grid-cols-2">
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

        <div v-else class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.modelPricings.customPricePerSecond') }}</label>
            <input
              v-model.number="editForm.custom_input_cost"
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

        <div>
          <label class="input-label">{{ t('admin.modelPricings.pricingUnitLabel') }}</label>
          <div class="mt-1 flex overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
            <label
              v-for="option in pricingUnitOptions"
              :key="option.value"
              class="flex-1 cursor-pointer px-3 py-2 text-center text-sm transition-colors"
              :class="
                createForm.pricing_unit === option.value
                  ? 'bg-primary-600 text-white'
                  : 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700'
              "
            >
              <input v-model="createForm.pricing_unit" type="radio" class="sr-only" :value="option.value" />
              {{ option.label }}
            </label>
          </div>
        </div>

        <div v-if="createForm.pricing_unit === 'token'" class="grid grid-cols-1 gap-4 md:grid-cols-2">
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

        <div v-else class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.modelPricings.upstreamPricePerSecond') }}</label>
            <input
              v-model.number="createForm.input_cost_per_token"
              type="number"
              step="any"
              min="0"
              class="input"
              placeholder="例: 0.12"
            />
          </div>
        </div>

        <div v-if="createForm.pricing_unit === 'token'" class="grid grid-cols-1 gap-4 md:grid-cols-2">
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

        <div v-if="createForm.pricing_unit === 'token'" class="grid grid-cols-1 gap-4 md:grid-cols-2">
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

        <div v-else class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.modelPricings.customPricePerSecond') }}</label>
            <input
              v-model.number="createForm.custom_input_cost"
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

    <!-- Clear discounts confirmation -->
    <ConfirmDialog
      :show="showClearDiscountsModal"
      title="清空所有折扣率"
      message="将把全表所有模型的折扣率清空（置为无折扣）。此操作不可撤销，确认继续？"
      confirm-text="清空"
      cancel-text="取消"
      danger
      @confirm="handleClearDiscounts"
      @cancel="showClearDiscountsModal = false"
    />

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

    <!-- Wanjie sync -->
    <BaseDialog :show="showWanjieModal" title="同步万界定价" width="wide" @close="closeWanjieModal">
      <form id="wanjie-sync-form" class="space-y-4" @submit.prevent="handleWanjieSync">
        <p class="text-sm text-gray-500 dark:text-gray-400">
          优先级：上传 JSON 文件 &gt; API URL + Token &gt; 已保存凭证 &gt; 内嵌离线数据。
        </p>

        <!-- File upload -->
        <div>
          <label class="input-label">上传 JSON 文件（最高优先级）</label>
          <div
            class="mt-1 flex cursor-pointer items-center gap-3 rounded-lg border-2 border-dashed border-gray-300 px-4 py-3 transition hover:border-primary-400 dark:border-dark-500 dark:hover:border-primary-500"
            @click="wanjieFileInputRef?.click()"
          >
            <Icon name="upload" size="md" class="shrink-0 text-gray-400" />
            <span class="truncate text-sm text-gray-500 dark:text-gray-400">
              {{ wanjieFileName || '点击选择 wanjie.json 文件' }}
            </span>
            <button
              v-if="wanjieFileName"
              type="button"
              class="ml-auto shrink-0 text-xs text-red-500 hover:text-red-700"
              @click.stop="clearWanjieFile"
            >
              清除
            </button>
          </div>
          <input
            ref="wanjieFileInputRef"
            type="file"
            accept="application/json,.json"
            class="hidden"
            @change="onWanjieFileChange"
          />
        </div>

        <div class="flex items-center gap-2 text-xs text-gray-400">
          <div class="h-px flex-1 bg-gray-200 dark:bg-dark-600" />
          <span>或</span>
          <div class="h-px flex-1 bg-gray-200 dark:bg-dark-600" />
        </div>

        <div>
          <label class="input-label">API URL</label>
          <input
            v-model="wanjieForm.url"
            type="url"
            class="input mt-1 font-mono"
            :disabled="!!wanjieFileName"
            placeholder="https://fangzhou.wanjiedata.com/maas/model/myModelList"
          />
        </div>
        <div>
          <label class="input-label">x-access-token</label>
          <input
            v-model="wanjieForm.access_token"
            type="password"
            autocomplete="off"
            class="input mt-1 font-mono"
            :disabled="!!wanjieFileName"
            placeholder="万界 JWT Token"
          />
        </div>
        <div class="flex items-center gap-2">
          <input id="wanjie-save-creds" v-model="wanjieForm.save_credentials" type="checkbox" class="h-4 w-4 rounded border-gray-300" :disabled="!!wanjieFileName" />
          <label for="wanjie-save-creds" class="text-sm text-gray-700 dark:text-gray-300" :class="wanjieFileName ? 'opacity-40' : ''">
            保存凭证到系统设置（下次同步自动使用）
          </label>
        </div>
        <p v-if="!wanjieFileName && !wanjieForm.url && !wanjieForm.access_token" class="text-xs text-amber-500 dark:text-amber-400">
          未填写任何内容时，将使用已保存的凭证或内嵌离线数据。
        </p>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeWanjieModal">取消</button>
          <button type="submit" form="wanjie-sync-form" :disabled="wanjieSyncing" class="btn btn-primary">
            {{ wanjieSyncing ? '同步中...' : '开始同步' }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Import from upstream -->
    <BaseDialog :show="showImportModal" title="从上游导入模型" width="wide" @close="closeImportModal">
      <form id="import-pricing-form" class="space-y-4" @submit.prevent="handleImport">
        <p class="text-sm text-gray-500 dark:text-gray-400">
          使用 Base URL + API Key 请求上游 <code>/v1/models</code>（兼容 newapi / OpenAI 变体），把模型写入定价表（已存在的不覆盖），随后可在此页设置折扣。
        </p>
        <div>
          <label class="input-label">Base URL</label>
          <input v-model="importForm.base_url" type="url" required class="input mt-1 font-mono" placeholder="https://maas-openapi.wanjiedata.com/api" />
        </div>
        <div>
          <label class="input-label">API Key</label>
          <input v-model="importForm.api_key" type="password" autocomplete="off" required class="input mt-1 font-mono" placeholder="上游渠道 API Key" />
        </div>
        <div>
          <label class="input-label">Provider（归属标识）</label>
          <input v-model="importForm.provider" type="text" required class="input mt-1 font-mono" placeholder="如 wanjie（区分不同上游，避免混桶）" />
          <p class="input-hint">用于按渠道归类与过滤，建议用渠道 slug，不要留空。</p>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="input-label">Auth Header（可选）</label>
            <input v-model="importForm.auth_header" type="text" class="input mt-1 font-mono" placeholder="Authorization" />
          </div>
          <div>
            <label class="input-label">Auth Scheme（可选）</label>
            <input v-model="importForm.auth_scheme" type="text" class="input mt-1 font-mono" placeholder="Bearer" />
          </div>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeImportModal">取消</button>
          <button type="submit" form="import-pricing-form" :disabled="importing" class="btn btn-primary">
            {{ importing ? '导入中...' : '导入' }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
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
  syncModelPricingsFromUpstream,
  syncModelPricingsFromWanjie,
  clearAllModelPricingDiscounts,
  listModelPricingProviders,
  type DBModelPricing,
  type CreateModelPricingRequest,
} from '@/api/admin/modelPricings'

const appStore = useAppStore()
const { t } = useI18n()

// ==================== State ====================

const loading = ref(false)
const saving = ref(false)
const syncing = ref(false)
const clearingDiscounts = ref(false)
const showClearDiscountsModal = ref(false)
const togglingId = ref<number | null>(null)

const items = ref<DBModelPricing[]>([])
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

const searchQuery = ref('')
const filterProvider = ref('')
const filterSource = ref('')
const filterEnabled = ref('')
const visibleOnly = ref(true)

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
  pricing_unit: 'token' | 'second'
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
  pricing_unit: 'token',
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
  pricing_unit: 'token' | 'second'
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
  pricing_unit: 'token',
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

// 从后端拉取的全部 provider（去重，按字母排序）。
const availableProviders = ref<string[]>([])

// 已知 provider 的展示名映射，其余按首字母大写兜底，避免显示成 "anthropic"/"openai" 这种小写。
const PROVIDER_LABEL_OVERRIDES: Record<string, string> = {
  anthropic: 'Anthropic',
  openai: 'OpenAI',
  google: 'Google',
  custom: 'Custom',
}

const formatProviderLabel = (provider: string): string => {
  const key = provider.toLowerCase()
  if (PROVIDER_LABEL_OVERRIDES[key]) return PROVIDER_LABEL_OVERRIDES[key]
  if (!provider) return provider
  return provider.charAt(0).toUpperCase() + provider.slice(1)
}

const providerOptions = computed(() => [
  { value: '', label: '全部提供商' },
  ...availableProviders.value.map(p => ({ value: p, label: formatProviderLabel(p) })),
])

const loadProviders = async () => {
  try {
    const { data } = await listModelPricingProviders()
    availableProviders.value = data.providers ?? []
  } catch {
    availableProviders.value = []
  }
}

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

const pricingUnitOptions = [
  { value: 'token', label: 'Token' },
  { value: 'second', label: t('admin.modelPricings.pricingUnitSecond') },
]

// ==================== Formatters ====================

const formatTokenPrice = (price: number | null | undefined): string => {
  if (price === null || price === undefined) return '—'
  const perMillion = price * 1_000_000
  if (perMillion === 0) return '$0.00 /M tok'
  // ≥ 0.01：2 位小数足够；< 0.01：保留至少 2 位有效数字，避免 0.1 折后 $0.0018 被 toFixed(2) 吞成 0.00
  const display = perMillion >= 0.01 ? perMillion.toFixed(2) : perMillion.toPrecision(2)
  return `$${display} /M tok`
}

const formatSecondPrice = (price: number | null | undefined): string => {
  if (price === null || price === undefined) return '—'
  return `${formatUsdAmount(price)}/${t('admin.modelPricings.secondUnit')}`
}

const formatDiscountRate = (rate: number | null | undefined): string => {
  if (rate === null || rate === undefined) return '—'
  return `${rate} (${(rate * 100).toFixed(0)}%)`
}

// 去掉末尾零、保留至多 6 位小数的 USD 美化输出。
const formatUsdAmount = (value: number): string => {
  return `$${value.toFixed(6).replace(/0+$/, '').replace(/\.$/, '')}`
}

// 图像 / 视频生成模型的价格不在 token 列：
//   - output_cost_per_image      —— "USD/张"（image）或 "USD/秒"（video）
//   - output_cost_per_image_token —— 灵境 seedance 这类 "USD/次请求"（已按预设时长预算）
// 单独走一个 formatter，避免被 formatTokenPrice 乘 1_000_000 显示成 $295,400 /M tok。
const formatPerUnitPrice = (price: number | null | undefined, mode: string): string => {
  if (price === null || price === undefined) return ''
  const unit = mode === 'video_generation' ? '秒' : '张'
  return `${formatUsdAmount(price)}/${unit}`
}

const formatPerRequestPrice = (price: number | null | undefined, mode: string): string => {
  if (price === null || price === undefined) return ''
  const unit = mode === 'video_generation' ? '视频' : '次'
  return `${formatUsdAmount(price)}/${unit}`
}

const formatUpstreamInputPrice = (item: DBModelPricing): string => {
  if (item.pricing_unit === 'second') {
    return formatSecondPrice(item.input_cost_per_token)
  }
  if (item.input_cost_per_token !== null && item.input_cost_per_token !== undefined) {
    return formatTokenPrice(item.input_cost_per_token)
  }
  return '—'
}

const formatUpstreamOutputPrice = (item: DBModelPricing): string => {
  if (item.pricing_unit === 'second') {
    return '—'
  }
  if (item.output_cost_per_token !== null && item.output_cost_per_token !== undefined) {
    return formatTokenPrice(item.output_cost_per_token)
  }
  const perUnit = formatPerUnitPrice(item.output_cost_per_image, item.mode)
  if (perUnit) return perUnit
  // 灵境 seedance 视频：output_cost_per_image_token 实际存的是"一次请求的总价"，不是 per-token 单价。
  const perRequest = formatPerRequestPrice(item.output_cost_per_image_token, item.mode)
  if (perRequest) return perRequest
  return '—'
}

const formatEffectiveInputPrice = (item: DBModelPricing): string => {
  const discount = item.discount_rate ?? 1
  const tokenBase = item.custom_input_cost ?? item.input_cost_per_token
  if (item.pricing_unit === 'second') {
    return formatSecondPrice(tokenBase !== null && tokenBase !== undefined ? tokenBase * discount : null)
  }
  if (tokenBase !== null && tokenBase !== undefined) {
    return formatTokenPrice(tokenBase * discount)
  }
  // image_generation / video_generation：使用 per-unit 价格直接乘以折扣率
  if (item.output_cost_per_image !== null && item.output_cost_per_image !== undefined) {
    return formatPerUnitPrice(item.output_cost_per_image * discount, item.mode)
  }
  if (item.output_cost_per_image_token !== null && item.output_cost_per_image_token !== undefined) {
    return formatPerRequestPrice(item.output_cost_per_image_token * discount, item.mode)
  }
  return '—'
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
    params.visible_only = visibleOnly.value

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

const handleClearDiscounts = async () => {
  showClearDiscountsModal.value = false
  clearingDiscounts.value = true
  try {
    await clearAllModelPricingDiscounts()
    appStore.showSuccess('已清空全表折扣率')
    await load()
  } catch {
    appStore.showError('清空折扣率失败')
  } finally {
    clearingDiscounts.value = false
  }
}

const handleSync = async () => {
  syncing.value = true
  try {
    await triggerModelPricingSync()
    appStore.showSuccess('同步触发成功，后台正在更新价格数据')
    await load()
    loadProviders()
  } catch {
    appStore.showError('同步失败')
  } finally {
    syncing.value = false
  }
}

// ==================== Wanjie sync ====================

const showWanjieModal = ref(false)
const wanjieSyncing = ref(false)
const wanjieFileInputRef = ref<HTMLInputElement | null>(null)
const wanjieFileName = ref('')
const wanjieJsonData = ref('')
const wanjieForm = reactive({
  url: '',
  access_token: '',
  save_credentials: false,
})

const onWanjieFileChange = (e: Event) => {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  wanjieFileName.value = file.name
  const reader = new FileReader()
  reader.onload = (ev) => {
    wanjieJsonData.value = (ev.target?.result as string) ?? ''
  }
  reader.readAsText(file)
}

const clearWanjieFile = () => {
  wanjieFileName.value = ''
  wanjieJsonData.value = ''
  if (wanjieFileInputRef.value) wanjieFileInputRef.value.value = ''
}

const openWanjieModal = () => {
  showWanjieModal.value = true
}

const closeWanjieModal = () => {
  showWanjieModal.value = false
  clearWanjieFile()
}

const handleWanjieSync = async () => {
  wanjieSyncing.value = true
  try {
    const { data } = await syncModelPricingsFromWanjie({
      json_data: wanjieJsonData.value || undefined,
      url: wanjieJsonData.value ? undefined : (wanjieForm.url.trim() || undefined),
      access_token: wanjieJsonData.value ? undefined : (wanjieForm.access_token.trim() || undefined),
      save_credentials: wanjieJsonData.value ? false : wanjieForm.save_credentials,
    })
    const srcMap: Record<string, string> = { upload: '上传文件', live: '实时 API', embedded: '内嵌离线数据' }
    const src = srcMap[data.source] ?? data.source
    appStore.showSuccess(`万界定价同步完成（来源：${src}），共处理 ${data.total} 个模型`)
    closeWanjieModal()
    await load()
    loadProviders()
  } catch (err) {
    const error = err as { response?: { data?: { message?: string; detail?: string } } }
    appStore.showError(error.response?.data?.message || error.response?.data?.detail || '万界定价同步失败')
  } finally {
    wanjieSyncing.value = false
  }
}

// ==================== Import from upstream ====================

const showImportModal = ref(false)
const importing = ref(false)
const importForm = reactive({
  base_url: '',
  api_key: '',
  provider: '',
  auth_header: '',
  auth_scheme: '',
})

const openImportModal = () => {
  importForm.base_url = ''
  importForm.api_key = ''
  importForm.provider = ''
  importForm.auth_header = ''
  importForm.auth_scheme = ''
  showImportModal.value = true
}

const closeImportModal = () => {
  showImportModal.value = false
}

const handleImport = async () => {
  if (!importForm.base_url.trim() || !importForm.api_key.trim() || !importForm.provider.trim()) {
    appStore.showError('请填写 Base URL、API Key 和 Provider')
    return
  }
  importing.value = true
  try {
    const { data } = await syncModelPricingsFromUpstream({
      base_url: importForm.base_url.trim(),
      api_key: importForm.api_key.trim(),
      provider: importForm.provider.trim(),
      auth_header: importForm.auth_header.trim() || undefined,
      auth_scheme: importForm.auth_scheme.trim() || undefined,
    })
    appStore.showSuccess(`已拉取 ${data.fetched ?? (data.models?.length ?? 0)} 个模型并写入定价表`)
    closeImportModal()
    await load()
    loadProviders()
  } catch (err) {
    const error = err as { response?: { data?: { message?: string; detail?: string } } }
    appStore.showError(error.response?.data?.message || error.response?.data?.detail || '导入失败')
  } finally {
    importing.value = false
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
  editForm.pricing_unit = item.pricing_unit ?? 'token'
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
      pricing_unit: editForm.pricing_unit,
      custom_input_cost: editForm.custom_input_cost ?? null,
      custom_output_cost: editForm.pricing_unit === 'token' ? (editForm.custom_output_cost ?? null) : null,
      discount_rate: editForm.discount_rate ?? null,
      is_enabled: editForm.is_enabled,
    }
    if (editingItem.value.is_custom) {
      payload.provider = editForm.provider
      payload.mode = editForm.mode
      payload.input_cost_per_token = editForm.input_cost_per_token ?? null
      payload.output_cost_per_token = editForm.pricing_unit === 'token' ? (editForm.output_cost_per_token ?? null) : null
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
  createForm.pricing_unit = 'token'
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
      pricing_unit: createForm.pricing_unit,
      input_cost_per_token: createForm.input_cost_per_token ?? null,
      output_cost_per_token: createForm.pricing_unit === 'token' ? (createForm.output_cost_per_token ?? null) : null,
      cache_creation_input_token_cost: createForm.pricing_unit === 'token' ? (createForm.cache_creation_input_token_cost ?? null) : null,
      cache_read_input_token_cost: createForm.pricing_unit === 'token' ? (createForm.cache_read_input_token_cost ?? null) : null,
      custom_input_cost: createForm.custom_input_cost ?? null,
      custom_output_cost: createForm.pricing_unit === 'token' ? (createForm.custom_output_cost ?? null) : null,
      discount_rate: createForm.discount_rate ?? null,
      is_enabled: createForm.is_enabled,
    }
    await createModelPricing(payload)
    appStore.showSuccess('创建成功')
    closeCreateModal()
    pagination.page = 1
    await load()
    loadProviders()
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

onMounted(() => {
  load()
  loadProviders()
})
</script>
