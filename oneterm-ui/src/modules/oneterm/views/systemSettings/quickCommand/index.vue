<template>
  <div class="quick-command">
    <a-spin :tip="loadTip" :spinning="loading">
      <div class="quick-command-header">
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
            @click="openModal(null)"
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
        :height="tableHeight"
        resizable
        @checkbox-change="onSelectChange"
        @checkbox-all="onSelectChange"
        @checkbox-range-end="onSelectChange"
      >
        <vxe-column type="checkbox" width="60px"></vxe-column>
        <vxe-column :title="$t('name')" field="name"> </vxe-column>
        <vxe-column :title="$t('description')" field="description"> </vxe-column>
        <vxe-column :title="$t('oneterm.command')" field="command"> </vxe-column>
        <vxe-column :title="$t(`operation`)" width="100">
          <template #default="{row}">
            <a-space>
              <a
                @click="openModal(row)"
              >
                <ops-icon type="icon-xianxing-edit"/>
              </a>
              <a-popconfirm
                :title="$t('confirmDelete')"
                @confirm="deleteCommpand(row.id)"
              >
                <a style="color:red">
                  <ops-icon type="icon-xianxing-delete"/>
                </a>
              </a-popconfirm>
            </a-space>
          </template>
        </vxe-column>
      </ops-table>
      <div class="quick-command-pagination">
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

    <CommandModal
      ref="commandModalRef"
      @ok="handleModalOk"
    />
  </div>
</template>

<script>
import { mapState } from 'vuex'
import { getQuickCommand, deleteQuickCommandById } from '@/modules/oneterm/api/quickCommand.js'

import CommandModal from './commandModal.vue'

export default {
  name: 'QuickCommand',
  components: {
    CommandModal
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
      return this.windowHeight - 205
    }
  },
  mounted() {
    this.updateTableData()
  },
  methods: {
    updateTableData() {
      this.loading = true
      getQuickCommand({
        page_index: this.currentPage,
        page_size: this.pageSize,
        search: this.filterName,
      })
        .then((res) => {
          this.tableData = res?.data?.list || []
          this.totalResult = res?.data?.count ?? 0
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

    deleteCommpand(id) {
      this.loading = true
      deleteQuickCommandById(id)
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
            await deleteQuickCommandById(this.selectedRowKeys[i], false)
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

    openModal(data) {
      this.$refs.commandModalRef.open(data)
    },

    handleModalOk() {
      this.currentPage = 1
      this.updateTableData()
    }
  }
}
</script>

<style lang="less" scoped>
.quick-command {
  padding: 18px;
  background-color: #ffffff;
  border-radius: 6px;

  &-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: 16px;
  }

  &-pagination {
    text-align: right;
    margin-top: 8px;
  }
}
</style>
