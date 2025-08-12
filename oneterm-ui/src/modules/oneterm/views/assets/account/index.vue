<template>
  <div class="oneterm-layout">
    <div class="oneterm-header">{{ $t('oneterm.menu.accountManagement') }}</div>
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
          <vxe-column :title="$t(`oneterm.password`) + ' | ' + $t('oneterm.secretkey')" field="account_type">
            <template #default="{row}">
              <div class="table-password">
                <template v-if="getPasswordText(row) && row.showPassword">
                  <a @click="row.showPassword = false"><a-icon type="eye" /></a>
                  <a @click="copyPassword(getPasswordText(row))"><a-icon type="copy" /></a>
                  <a-tooltip
                    :title="getPasswordText(row)"
                    :overlayStyle="{
                      overflow: 'auto',
                      maxHeight: '400px'
                    }"
                  >
                    <span>{{ getPasswordText(row) }}</span>
                  </a-tooltip>
                </template>
                <template v-else>
                  <a
                    v-if="getAccountPermission(row, 'read')"
                    @click="showTablePassword(row)"
                  >
                    <a-icon type="eye-invisible" />
                  </a>
                  <span>******</span>
                </template>
              </div>
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
                <a @click="clickEditButton(row)" v-if="getAccountPermission(row, 'write')" ><ops-icon type="icon-xianxing-edit"/></a>
                <a-popconfirm :title="$t('confirmDelete')" v-if="getAccountPermission(row, 'delete')" @confirm="deleteGateway(row)">
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
import { getAccountList, deleteAccountById, getAccountByCredentials } from '@/modules/oneterm/api/account'
import { getAllDepAndEmployee } from '@/api/company'

import GrantModal from '@/modules/oneterm/components/grant/grantModal.vue'
import AccountModal from './accountModal.vue'

export default {
  name: 'Account',
  components: {
    AccountModal,
    GrantModal
  },
  provide() {
    return {
      provide_allTreeDepAndEmp: () => {
        return this.allTreeDepAndEmp
      },
    }
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
      allTreeDepAndEmp: []
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
    getAllDepAndEmployee({ block: 0 }).then((res) => {
      this.allTreeDepAndEmp = res
    })
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
          const tableData = res?.data?.list || []
          tableData.forEach((item) => {
            item.showPassword = false
          })
          this.tableData = tableData
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
    openModal() {
      this.$refs.accountModal.open()
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

    getAccountPermission(account, operation) {
      const permissions = this?.roles?.permissions || []
      const isAdmin = permissions?.includes?.('oneterm_admin') || permissions?.includes?.('acl_admin')
      return account?.permissions?.some((perm) => perm === operation) || isAdmin
    },

    getPasswordText(data) {
      return data.account_type === 1 ? data.password : data.pk
    },

    async showTablePassword(data) {
      if (this.getPasswordText(data)) {
        data.showPassword = true
      } else {
        const res = await getAccountByCredentials(data.id)
        if (res?.data) {
          const accountData = this.handleTablePassword(res.data)
          this.$set(accountData, 'showPassword', true)
        }
      }
    },

    async clickEditButton(data) {
      if (this.getPasswordText(data)) {
        this.$refs.accountModal.open(data)
      } else {
        const res = await getAccountByCredentials(data.id)
        if (res?.data) {
          const accountData = this.handleTablePassword(res.data)
          this.$refs.accountModal.open(accountData)
        }
      }
    },

    handleTablePassword(data) {
      let newData = data
      const tableDataIndex = this.tableData.findIndex((item) => item.id === data.id)

      if (tableDataIndex !== -1) {
        const rowData = this.tableData[tableDataIndex]
        const { pk, password, phrase, account_type } = data
        if (account_type === 1) {
          rowData.password = password
        } else {
          rowData.pk = pk
          rowData.phrase = phrase
        }

        this.$set(this.tableData, tableDataIndex, rowData)
        newData = rowData
      }

      return newData
    },

    copyPassword(text) {
      this.$copyText(text)
        .then(() => {
          this.$message.success(this.$t('copySuccess'))
        })
    }
  },
}
</script>

<style lang="less" scoped>
@import '../../../style/index.less';

.table-password {
  display: flex;
  align-items: center;
  overflow: hidden;

  a {
    margin-right: 8px;
    flex-shrink: 0;
  }

  span {
    width: 100%;
    text-overflow: ellipsis;
    overflow: hidden;
    text-wrap: nowrap;
  }
}
</style>
