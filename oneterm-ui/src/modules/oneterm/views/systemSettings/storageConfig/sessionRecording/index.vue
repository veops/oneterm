<template>
  <div class="session-recording">
    <a-spin :tip="loadTip" :spinning="loading">
      <div class="session-recording-header">
        <a-space>
          <a-input-search
            v-model="filterName"
            allow-clear
            :style="{ width: '250px' }"
            class="ops-input ops-input-radius"
            :placeholder="$t('placeholderSearch')"
            @search="updateTableData()"
          />
          <div
            class="ops-list-batch-action"
            v-show="!!selectedRowKeys.length"
          >
            <span @click="batchDelete">{{ $t(`delete`) }}</span>
            <span>{{ $t('selectRows', { rows: selectedRowKeys.length }) }}</span>
          </div>
        </a-space>
        <a-space>
          <a-button
            type="primary"
            @click="openDrawer(null)"
          >
            {{ $t(`create`) }}
          </a-button>
          <a-button
            @click="updateTableData()"
          >
            {{ $t(`refresh`) }}
          </a-button>
        </a-space>
      </div>
      <ops-table
        size="small"
        ref="opsTable"
        stripe
        class="ops-stripe-table"
        :data="tableData"
        show-overflow
        show-header-overflow
        :checkbox-config="{ reserve: true, highlight: true, range: true }"
        :row-config="{ keyField: 'id' }"
        :column-config="{ minWidth: '100' }"
        :height="tableHeight"
        resizable
        @checkbox-change="onSelectChange"
        @checkbox-all="onSelectChange"
        @checkbox-range-end="onSelectChange"
      >
        <vxe-column type="checkbox" width="60px"></vxe-column>
        <vxe-column :title="$t('name')" field="name"> </vxe-column>
        <vxe-column :title="$t('type')" field="type"> </vxe-column>
        <vxe-column :title="$t('description')" field="description"> </vxe-column>
        <vxe-column :title="$t('oneterm.storageConfig.priority')" field="priority"></vxe-column>
        <vxe-column :title="$t('oneterm.storageConfig.healthStatus')" field="healthStatus">
          <template #default="{row}">
            <div class="health-status-yes" v-if="row.healthStatus">
              <a-icon class="health-status-yes-icon" type="check-circle" theme="filled" />
              <div class="health-status-yes-text">{{ $t('oneterm.storageConfig.normal') }}</div>
            </div>
            <a-tooltip
              v-else
              :title="row.healthErrorMessage"
            >
              <div class="health-status-no">
                <a-icon class="health-status-no-icon" type="close-circle" theme="filled" />
                <div class="health-status-no-text">{{ $t('oneterm.storageConfig.abnormal') }}</div>
              </div>
            </a-tooltip>
          </template>
        </vxe-column>
        <vxe-column :title="$t('oneterm.storageConfig.isPrimary')" field="is_primary" min-width="70">
          <template #default="{row}">
            <a-popconfirm
              :title="$t('oneterm.storageConfig.confirmPrimaryStorage')"
              @confirm="switchPrimaryStorage(row)"
            >
              <a-switch :checked="row.is_primary"/>
            </a-popconfirm>
          </template>
        </vxe-column>
        <vxe-column :title="$t('oneterm.isEnable')" field="enabled" min-width="70">
          <template #default="{row}">
            <a-popconfirm
              :title="$t('oneterm.confirmEnable')"
              @confirm="toggleEnabled(row)"
            >
              <a-switch :checked="row.enabled" />
            </a-popconfirm>
          </template>
        </vxe-column>
        <vxe-column :title="$t(`operation`)" width="100" fixed="right">
          <template #default="{row}">
            <a-space>
              <a
                @click="openDrawer(row)"
              >
                <ops-icon type="icon-xianxing-edit"/>
              </a>
              <a-popconfirm
                :title="$t('confirmDelete')"
                @confirm="deleteConfig(row.id)"
              >
                <a style="color:red">
                  <ops-icon type="icon-xianxing-delete"/>
                </a>
              </a-popconfirm>
            </a-space>
          </template>
        </vxe-column>
      </ops-table>
      <div class="session-recording-pagination">
        <a-pagination
          size="small"
          show-size-changer
          :current="currentPage"
          :total="totalResult"
          :show-total="
            (total, range) =>
              $t('pagination.total', {
                range0: range[0],
                range1: range[1],
                total,
              })
          "
          :page-size="pageSize"
          :default-current="1"
          :page-size-options="pageSizeOptions"
          @change="pageOrSizeChange"
          @showSizeChange="pageOrSizeChange"
        />
      </div>
    </a-spin>

    <ConfigDrawer
      ref="configDrawerRef"
      @ok="handleDrawerOk"
    />
  </div>
</template>

<script>
import { mapState } from 'vuex'
import {
  getStorageConfigs,
  deleteStorageConfigs,
  setPrimaryStorageConfig,
  toggleEnabled,
  getStorageHealth
} from '@/modules/oneterm/api/storage.js'

import ConfigDrawer from './configDrawer.vue'

export default {
  name: 'SessionRecording',
  components: {
    ConfigDrawer
  },
  data() {
    return {
      filterName: '',
      pageSizeOptions: ['20', '50', '100', '200'],
      currentPage: 1,
      pageSize: 20,
      totalResult: 0,
      tableData: [],
      selectedRowKeys: [],

      loading: false,
      loadTip: '',
    }
  },
  computed: {
    ...mapState({
      windowHeight: (state) => state.windowHeight,
    }),
    tableHeight() {
      return this.windowHeight - 245
    }
  },
  mounted() {
    this.updateTableData()
  },
  methods: {
    updateTableData() {
      this.loading = true
      Promise.all([
        getStorageConfigs({
          page_index: this.currentPage,
          page_size: this.pageSize,
          search: this.filterName,
        }),
        getStorageHealth()
      ]).then((res) => {
          const [tableRes, healthRes] = res
          const healthMap = healthRes?.data
          const tableData = tableRes?.data?.list || []
          tableData.forEach((item) => {
            const currentHealth = healthMap?.[item?.name] || {}
            item.healthStatus = currentHealth?.healthy ?? false
            if (!item.healthStatus && currentHealth?.error) {
              item.healthErrorMessage = currentHealth?.error
            }
          })
          this.tableData = tableData
          this.totalResult = tableRes?.data?.count ?? 0
        })
        .finally(() => {
          this.loading = false
        })
    },

    pageOrSizeChange(currentPage, pageSize) {
      this.currentPage = currentPage
      this.pageSize = pageSize
      this.updateTableData()
    },

    onSelectChange({ records }) {
      this.selectedRowKeys = records.map((i) => i.id)
    },

    async toggleEnabled(row) {
      await toggleEnabled(row.id)
      this.updateTableData()
    },

    async switchPrimaryStorage(row) {
      await setPrimaryStorageConfig(row.id)
      this.updateTableData()
    },

    deleteConfig(id) {
      this.loading = true
      deleteStorageConfigs(id)
        .then(() => {
          this.$message.success(this.$t('deleteSuccess'))
          this.updateTableData()
        })
        .finally(() => {
          this.loading = false
        })
    },

    async batchDelete() {
      this.$confirm({
        title: this.$t('warning'),
        content: this.$t('confirmDelete'),
        onOk: async () => {
          let successNum = 0
          let errorNum = 0
          this.loading = true
          this.loadTip = `${this.$t('deleting')}...`
          for (let i = 0; i < this.selectedRowKeys.length; i++) {
            await deleteStorageConfigs(this.selectedRowKeys[i])
              .then(() => {
                successNum += 1
              })
              .catch(() => {
                errorNum += 1
              })
              .finally(() => {
                this.loadTip = this.$t('deletingTip', { total: this.selectedRowKeys.length, successNum, errorNum })
              })
          }
          this.loading = false
          this.loadTip = ''
          this.selectedRowKeys = []
          this.$refs.opsTable.getVxetableRef().clearCheckboxRow()
          this.$refs.opsTable.getVxetableRef().clearCheckboxReserve()
          this.$nextTick(() => {
            this.updateTableData()
          })
        },
      })
    },

    openDrawer(data) {
      this.$refs.configDrawerRef.open(data)
    },

    handleDrawerOk() {
      this.currentPage = 1
      this.updateTableData()
    }
  }
}
</script>

<style lang="less" scoped>
.session-recording {
  padding: 18px;
  background-color: #ffffff;
  border-radius: 6px;

  &-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: 16px;
  }

  .health-status-yes {
    padding: 4px 7px;
    border-radius: 1px;
    line-height: 14px;
    background-color: #DCF3E3;
    display: inline-flex;
    align-items: center;
    justify-content: center;

    &-icon {
      font-size: 12px;
      color: #00B42A;
    }

    &-text {
      font-size: 12px;
      font-weight: 400;
      color: #30AD2D;
      margin-left: 4px;
    }
  }

  .health-status-no {
    padding: 0px 7px;
    border-radius: 1px;
    background-color: #E4E7ED;
    display: inline-flex;
    align-items: center;
    justify-content: center;

    &-icon {
      font-size: 12px;
      color: #A5A9BC;
    }

    &-text {
      font-size: 12px;
      font-weight: 400;
      color: #4E5969;
      margin-left: 4px;
    }
  }

  &-pagination {
    text-align: right;
    margin-top: 8px;
  }
}
</style>
