<template>
  <CustomDrawer
    :visible="visible"
    :title="$t('oneterm.workStation.recentSession')"
    width="920px"
    :bodyStyle="{
      height: '100%'
    }"
    @close="handleCancel"
  >
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
          <vxe-column :title="$t(`oneterm.workStation.loginTime`)" field="created_at">
            <template #default="{row}">
              {{ moment(row.created_at).format('YYYY-MM-DD HH:mm:ss') }}
            </template>
          </vxe-column>
          <vxe-column :title="$t(`operation`)" width="80" align="center">
            <template #default="{row}">
              <a-space>
                <a-tooltip :title="row.protocolType">
                  <a @click="openTerminal(row)"><ops-icon :type="row.protocolIcon"/></a>
                </a-tooltip>
                <a-tooltip :title="$t(`oneterm.switchAccount`)">
                  <a @click="openLogin(row)"><ops-icon type="oneterm-switch"/></a>
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
      <LoginModal
        ref="loginModal"
        @openTerminal="loginOpenTerminal"
      />
    </div>
  </CustomDrawer>
</template>

<script>
import moment from 'moment'
import { mapGetters, mapState } from 'vuex'
import { getSessionList } from '../../api/session'
import { getAssetList } from '../../api/asset'
import LoginModal from '../assets/assets/loginModal.vue'
export default {
  name: 'RecentSession',
  components: { LoginModal },
  data() {
    return {
      visible: false,
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
    ...mapState({
      rid: (state) => state.user.rid,
      roles: (state) => state.user.roles,
    }),
  },
  methods: {
    moment,
    open() {
      this.visible = true
      this.updateTableData()
    },
    handleCancel() {
      this.visible = false
      this.tableData = []
    },
    updateTableData(currentPage = 1, pageSize = this.tablePage.pageSize) {
      this.loading = true
      getSessionList({
        page_index: currentPage,
        page_size: pageSize,
        uid: this.uid,
      })
        .then((res) => {
          const protocolIconMap = {
            'ssh': 'a-oneterm-ssh2',
            'rdp': 'a-oneterm-ssh1',
            'vnc': 'oneterm-rdp',
          }

          const tableData = res?.data?.list || []
          tableData.forEach((item) => {
            const protocolType = item.protocol.split?.(':')?.[0] || ''
            item.protocolIcon = protocolIconMap?.[protocolType] || ''
            item.protocolType = protocolType
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
    pageOrSizeChange(currentPage, pageSize) {
      this.updateTableData(currentPage, pageSize)
    },
    async openTerminal(row) {
      const res = await getAssetList({
        id: row.asset_id,
        info: true
      })
      const asset = (res?.data?.list || [])?.[0]

      const protocolType = row.protocol.split?.(':')?.[0] || ''

      this.$emit('openTerminal', {
        assetId: row.asset_id,
        assetName: asset?.name || row?.asset_info || '',
        accountId: row.account_id,
        protocol: row.protocol,
        protocolType
      })
      this.handleCancel()
    },

    openLogin(row) {
      getAssetList({
        id: row.asset_id,
        info: true
      }).then((res) => {
        const asset = (res?.data?.list || [])?.[0]
        const accountLength = Object.keys(asset?.authorization || {})?.length

        if (accountLength) {
          this.$refs.loginModal.open(row.asset_id, asset?.name || '', asset.authorization, asset.protocols)
        } else {
          this.$message.warning(this.$t('oneterm.sessionTable.loginMessage'))
        }
      })
    },

    loginOpenTerminal(data) {
      this.$emit('openTerminal', data)
      this.handleCancel()
    }
  },
}
</script>

<style lang="less" scoped>
.recent-session {
  height: 100%;
  .recent-session-table {
    height: calc(100% - 32.5px);
  }
  .recent-session-pagination {
    margin-top: 8px;
    text-align: right;
  }
}
</style>
