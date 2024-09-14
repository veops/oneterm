<template>
  <div class="assets-command">
    <a-spin :tip="loadTip" :spinning="loading">
      <div class="assets-command-header">
        <a-space>
          <a-input-search
            allow-clear
            v-model="filterName"
            :style="{ width: '250px' }"
            class="ops-input ops-input-radius"
            :placeholder="$t('placeholderSearch')"
            @search="updateTableData()"
          />
          <div class="ops-list-batch-action" v-show="!!selectedRowKeys.length">
            <span @click="batchDelete">{{ $t(`delete`) }}</span>
            <span>{{ $t('selectRows', { rows: selectedRowKeys.length }) }}</span>
          </div>
        </a-space>
        <a-space>
          <a-button type="primary" @click="openModal(null)">{{ $t(`create`) }}</a-button>
          <a-button @click="updateTableData()">{{ $t(`refresh`) }}</a-button>
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
        @checkbox-change="onSelectChange"
        @checkbox-all="onSelectChange"
        @checkbox-range-end="onSelectRangeEnd"
        :checkbox-config="{ reserve: true, highlight: true, range: true }"
        :row-config="{ keyField: 'id' }"
        :height="tableHeight"
        resizable
      >
        <vxe-column type="checkbox" width="60px"></vxe-column>
        <vxe-column :title="$t(`oneterm.name`)" field="name"> </vxe-column>
        <vxe-column :title="$t(`oneterm.command`)" field="cmd"></vxe-column>
        <vxe-column :title="$t(`oneterm.assetList.enable`)" field="enable">
          <template #default="{row}">
            <a-switch :checked="Boolean(row.enable)" @change="changeEnable(row)" />
          </template>
        </vxe-column>
        <vxe-column :title="$t(`created_at`)" width="120">
          <template #default="{row}">
            {{ moment(row.created_at).format('YYYY-MM-DD') }}
          </template>
        </vxe-column>
        <vxe-column :title="$t(`operation`)" width="100">
          <template #default="{row}">
            <a-space>
              <a @click="openModal(row)"><ops-icon type="icon-xianxing-edit"/></a>
              <a-popconfirm :title="$t('confirmDelete')" @confirm="deleteCommand(row)">
                <a style="color:red"><ops-icon type="icon-xianxing-delete"/></a>
              </a-popconfirm>
            </a-space>
          </template>
        </vxe-column>
      </ops-table>
      <div class="assets-command-pagination">
        <a-pagination
          size="small"
          show-size-changer
          :current="tablePage.currentPage"
          :total="tablePage.totalResult"
          :show-total="
            (total, range) =>
              $t('pagination.total', {
                range0: range[0],
                range1: range[1],
                total,
              })
          "
          :page-size="tablePage.pageSize"
          :default-current="1"
          @change="pageOrSizeChange"
          @showSizeChange="pageOrSizeChange"
        />
      </div>
    </a-spin>
    <CommandModal ref="commandModal" @submit="updateTableData()" />
  </div>
</template>

<script>
import moment from 'moment'
import { mapState } from 'vuex'
import CommandModal from './commandModal.vue'
import { getCommandList, deleteCommandById, putCommandById } from '../../../api/command'

export default {
  name: 'Command',
  components: { CommandModal },
  data() {
    return {
      filterName: '',
      tableData: [],
      tablePage: {
        currentPage: 1,
        pageSize: 20,
        totalResult: 0,
      },
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
      return this.windowHeight - 248
    },
  },
  mounted() {
    this.updateTableData()
  },
  methods: {
    moment,
    updateTableData(currentPage = 1, pageSize = this.tablePage.pageSize) {
      this.loading = true
      getCommandList({
        page_index: currentPage,
        page_size: pageSize,
        search: this.filterName,
      })
        .then((res) => {
          this.tableData = res?.data?.list || []
          this.tablePage = {
            ...this.tablePage,
            currentPage,
            pageSize,
            totalResult: res?.data?.count ?? 0,
          }
        })
        .finally(() => {
          this.loading = false
        })
    },
    onSelectChange() {
      const opsTable = this.$refs.opsTable.getVxetableRef()
      const records = [...opsTable.getCheckboxRecords(), ...opsTable.getCheckboxReserveRecords()]
      this.selectedRowKeys = records.map((i) => i.id)
    },
    onSelectRangeEnd({ records }) {
      this.selectedRowKeys = records.map((i) => i.id)
    },
    pageOrSizeChange(currentPage, pageSize) {
      this.updateTableData(currentPage, pageSize)
    },
    openModal(data) {
      this.$refs.commandModal.open(data)
    },
    deleteCommand(row) {
      this.loading = true
      deleteCommandById(row.id)
        .then((res) => {
          this.$message.success(this.$t('deleteSuccess'))
          this.updateTableData()
        })
        .finally(() => {
          this.loading = false
        })
    },
    async batchDelete() {
      const that = this
      this.$confirm({
        title: that.$t('warning'),
        content: that.$t('confirmDelete'),
        async onOk() {
          let successNum = 0
          let errorNum = 0
          that.loading = true
          that.loadTip = `${that.$t('deleting')}...`
          for (let i = 0; i < that.selectedRowKeys.length; i++) {
            await deleteCommandById(that.selectedRowKeys[i], false)
              .then(() => {
                successNum += 1
              })
              .catch(() => {
                errorNum += 1
              })
              .finally(() => {
                that.loadTip = that.$t('deletingTip', { total: that.selectedRowKeys.length, successNum, errorNum })
              })
          }
          that.loading = false
          that.loadTip = ''
          that.selectedRowKeys = []
          that.$refs.opsTable.getVxetableRef().clearCheckboxRow()
          that.$refs.opsTable.getVxetableRef().clearCheckboxReserve()
          that.$nextTick(() => {
            that.updateTableData()
          })
        },
      })
    },
    changeEnable(row) {
      putCommandById(row.id, { ...row, enable: Boolean(!row.enable) }).then(() => {
        this.$message.success(this.$t('editSuccess'))
        this.updateTableData()
      })
    },
  },
}
</script>

<style lang="less" scoped>
@import '../../../style/index.less';
.assets-command {
  background-color: #fff;
  height: calc(100vh - 48px - 40px - 40px);
  border-bottom-left-radius: 15px;
  border-bottom-right-radius: 15px;
  padding: 18px;
  .assets-command-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: 16px;
  }
  .assets-command-pagination {
    text-align: right;
    margin-top: 8px;
  }
}
</style>
