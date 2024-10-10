<template>
  <div class="oneterm-layout">
    <div class="oneterm-header">{{ $t('oneterm.menu.accounts') }}</div>
    <a-spin :tip="loadTip" :spinning="loading">
      <div class="oneterm-layout-container">
        <div class="oneterm-layout-container-header">
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
              <span @click="grantAccount">{{ $t(`grant`) }}</span>
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
          <vxe-column :title="$t(`oneterm.account`)" field="account"> </vxe-column>
          <vxe-column :title="$t(`oneterm.accountType`)" field="account_type">
            <template #default="{row}">
              <a-space
                v-if="row.account_type === 1"
              ><ops-icon type="oneterm-password" /><span>{{ $t('oneterm.password') }}</span></a-space
              >
              <a-space
                v-else
              ><ops-icon type="oneterm-secret_key" /><span>{{ $t('oneterm.secretkey') }}</span></a-space
              >
            </template>
          </vxe-column>
          <vxe-column :title="$t(`oneterm.assetCount`)" field="asset_count"> </vxe-column>
          <vxe-column :title="$t(`created_at`)" width="120">
            <template #default="{row}">
              {{ moment(row.created_at).format('YYYY-MM-DD') }}
            </template>
          </vxe-column>
          <vxe-column :title="$t(`operation`)" width="100">
            <template #default="{row}">
              <a-space>
                <a @click="openModal(row)" v-if="showAccountOperation(row, 'write')" ><ops-icon type="icon-xianxing-edit"/></a>
                <a-popconfirm :title="$t('confirmDelete')" v-if="showAccountOperation(row, 'delete')" @confirm="deleteGateway(row)">
                  <a style="color:red"><ops-icon type="icon-xianxing-delete"/></a>
                </a-popconfirm>
              </a-space>
            </template>
          </vxe-column>
        </ops-table>
        <div class="oneterm-layout-pagination">
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
      </div>
    </a-spin>
    <AccountModal ref="accountModal" @submit="updateTableData()" />
    <GrantModal ref="grantModalRef" />
  </div>
</template>

<script>
import moment from 'moment'
import { mapState } from 'vuex'
import AccountModal from './accountModal.vue'
import { getAccountList, deleteAccountById } from '../../../api/account'
import GrantModal from '@/modules/oneterm/components/grant/grantModal.vue'

export default {
  name: 'Account',
  components: {
    AccountModal,
    GrantModal
  },
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
      roles: (state) => state.user.roles,
    }),
    tableHeight() {
      return this.windowHeight - 258
    },
  },
  mounted() {
    this.updateTableData()
  },
  methods: {
    moment,
    updateTableData(currentPage = 1, pageSize = this.tablePage.pageSize) {
      this.loading = true
      getAccountList({
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
      this.$refs.accountModal.open(data)
    },
    deleteGateway(row) {
      this.loading = true
      deleteAccountById(row.id)
        .then(() => {
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
            await deleteAccountById(that.selectedRowKeys[i], false)
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

    grantAccount() {
      const firstSlect = this.tableData?.find((item) => item.id === this.selectedRowKeys?.[0]) || {}

      this.$refs.grantModalRef.open({
        resourceId: firstSlect?.resource_id || '',
        type: 'account',
        ids: [...this.selectedRowKeys]
      })
    },

    showAccountOperation(account, operation) {
      const permissions = this?.roles?.permissions || []
      const isAdmin = permissions?.includes?.('oneterm_admin') || permissions?.includes?.('acl_admin')
      return account?.permissions?.some((perm) => perm === operation) || isAdmin
    }
  },
}
</script>

<style lang="less" scoped>
@import '../../../style/index.less';
</style>
