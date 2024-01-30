<template>
  <div class="recent-session">
    <div class="recent-session-table">
      <ops-table
        :loading="loading"
        size="mini"
        ref="opsTable"
        stripe
        class="ops-stripe-table"
        :data="tableData"
        show-overflow
        show-header-overflow
        :row-config="{ keyField: 'id' }"
        height="auto"
        resizable
      >
        <vxe-column :title="$t(`oneterm.sessionTable.target`)" field="asset_info"> </vxe-column>
        <vxe-column :title="$t(`oneterm.account`)" field="account_info"> </vxe-column>
        <vxe-column :title="$t(`oneterm.sessionTable.clientIp`)" field="client_ip"> </vxe-column>
        <vxe-column :title="$t(`oneterm.protocol`)" field="protocol"> </vxe-column>
        <vxe-column :title="$t(`operation`)" width="80" align="center">
          <template #default="{ row }">
            <a-space>
              <a-tooltip :title="$t(`login`)">
                <a @click="openTerminal(row)"><ops-icon type="oneterm-login" /></a>
              </a-tooltip>
              <a-tooltip :title="$t(`oneterm.switchAccount`)">
                <a @click="openLogin(row)"><ops-icon type="oneterm-switch" /></a>
              </a-tooltip>
            </a-space>
          </template>
        </vxe-column>
      </ops-table>
    </div>
    <div class="recent-session-pagination">
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
    <LoginModal ref="loginModal" />
  </div>
</template>

<script>
import { mapGetters } from 'vuex'
import { getSessionList } from '../../api/session'
import { getAssetList } from '../../api/asset'
import { postConnectIsRight } from '../../api/connect'
import LoginModal from '../assets/assets/loginModal.vue'
export default {
  name: 'RecentSession',
  components: { LoginModal },
  data() {
    return {
      tableData: [],
      tablePage: {
        currentPage: 1,
        pageSize: 20,
        totalResult: 0,
      },
      loading: false,
    }
  },
  computed: {
    ...mapGetters(['uid']),
  },
  mounted() {
    this.updateTableData()
  },
  methods: {
    updateTableData(currentPage = 1, pageSize = this.tablePage.pageSize) {
      this.loading = true
      getSessionList({
        page_index: currentPage,
        page_size: pageSize,
        uid: this.uid,
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
    pageOrSizeChange(currentPage, pageSize) {
      this.updateTableData(currentPage, pageSize)
    },
    openTerminal(row) {
      postConnectIsRight(row.asset_id, row.account_id, row.protocol).then((res) => {
        if (res?.data?.session_id) {
          window.open(`/oneterm/terminal?session_id=${res?.data?.session_id}`, '_blank')
        }
      })
    },
    openLogin(row) {
      getAssetList({ id: row.asset_id }).then((res) => {
        const asset = (res?.data?.list || [])[0]
        if (asset) {
          this.$refs.loginModal.open(row.asset_id, asset.authorization, asset.protocols)
        } else {
          this.$message.warning(this.$t('oneterm.sessionTable.loginMessage'))
        }
      })
    },
  },
}
</script>

<style lang="less" scoped>
.recent-session {
  padding: 10px;
  .recent-session-table {
    height: calc(100% - 32.5px);
  }
  .recent-session-pagination {
    margin-top: 8px;
    text-align: right;
  }
}
</style>
