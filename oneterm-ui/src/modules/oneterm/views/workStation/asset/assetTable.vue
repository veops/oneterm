<template>
  <div class="workstation-asset-table">
    <div class="workstation-asset-table-header">
      <a-input-search
        allow-clear
        v-model="filterName"
        :style="{ width: '250px' }"
        :placeholder="$t('placeholderSearch')"
        @search="updateTableData"
      />
      <a-space>
        <a-button
          type="primary"
          class="ops-button-ghost"
          ghost
          @click="updateTableData"
        >
          <ops-icon type="veops-refresh" />
          {{ $t('refresh') }}
        </a-button>
      </a-space>
    </div>
    <ops-table
      size="small"
      ref="opsTable"
      show-overflow
      show-header-overflow
      :height="`${windowHeight - 252}px`"
      resizable
      :data="tableData"
      :loading="loading"
      :checkbox-config="{ reserve: true, highlight: true, range: true }"
      :expand-config="{iconOpen: 'vxe-icon-square-minus', iconClose: 'vxe-icon-square-plus'}"
      :row-config="{ keyField: 'id', isHover: true }"
      @cell-click="onCellClick"
    >
      <vxe-column :type="'expand'" :title="$t(`oneterm.name`)" field="name">
        <template #default="{ row }">
          <span>{{ row.name }}</span>
        </template>
        <template #content="{ row }">
          <div v-if="row.accountList.length" class="workstation-asset-table-account">
            <div
              v-for="(item) in row.accountList"
              :key="item.protocol + item.account_id"
              class="workstation-asset-table-account-item"
              @click="openTerminal(row.id, row.name, item)"
            >
              <ops-icon class="workstation-asset-table-account-protocol" :type="item.protocolIcon" />
              <span class="workstation-asset-table-account-name">{{ item.account_name }}</span>
            </div>
          </div>
        </template>
      </vxe-column>
      <vxe-column :title="$t(`oneterm.assetList.ip`)" field="ip"> </vxe-column>
      <vxe-column :title="$t(`oneterm.assetList.catalogName`)" field="node_chain"> </vxe-column>
      <vxe-column
        :title="$t(`status`)"
        field="connectable"
        align="center"
        min-width="105px"
      >
        <template #default="{row}">
          <span class="workstation-asset-table-status workstation-asset-table-right" v-if="row.connectable">{{ $t(`oneterm.assetList.online`) }}</span>
          <span class="workstation-asset-table-status workstation-asset-table-error" v-else>{{ $t(`oneterm.assetList.offline`) }}</span>
        </template>
      </vxe-column>
      <vxe-column :title="$t(`operation`)" :width="100" align="center">
        <template #default="{row}">
          <a-space v-if="row.accountList.length">
            <a-tooltip
              v-for="(item) in row._protocols"
              :key="item.key"
              :title="item.key"
            >
              <a
                class="workstation-asset-table-operation-btn"
                @click="clickProtocol(item, row)"
              >
                <ops-icon v-if="item.icon" :type="item.icon" />
              </a>
            </a-tooltip>
          </a-space>
        </template>
      </vxe-column>
    </ops-table>
    <div class="workstation-asset-table-pagination">
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
        @change="pageOrSizeChange"
        @showSizeChange="pageOrSizeChange"
      />
    </div>

    <LoginModal
      ref="loginModalRef"
      :showProtocol="false"
      :choiceAccountByCheckbox="true"
      :getRequestParams="getRequestParams"
      @openTerminal="loginOpenTerminal"
      @openTerminalList="loginOpenTerminalList"
    />
  </div>
</template>

<script>
import { mapState } from 'vuex'
import { getAssetList } from '@/modules/oneterm/api/asset'

import LoginModal from '@/modules/oneterm/views/assets/assets/loginModal.vue'

export default {
  name: 'AssetTable',
  components: {
    LoginModal
  },
  props: {
    selectedKeys: {
      type: Array,
      default: () => []
    },
    accountList: {
      type: Array,
      default: () => []
    }
  },
  data() {
    return {
      tableData: [],
      filterName: '',
      currentPage: 1,
      pageSize: 20,
      totalResult: 0,
      loading: false,
      getRequestParams: {
        info: false
      }
    }
  },
  computed: {
    ...mapState({
      windowHeight: (state) => state.windowHeight,
    }),
  },
  watch: {
    selectedKeys: {
      immediate: true,
      deep: true,
      handler() {
        this.currentPage = 1
        this.updateTableData()
      },
    },
  },
  methods: {
    updateTableData() {
      this.loading = true
      getAssetList({
        parent_id: this.selectedKeys[0],
        page_index: this.currentPage,
        page_size: this.pageSize,
        search: this.filterName,
        info: this.getRequestParams.info,
      })
        .then(async (res) => {
          const protocolIconMap = {
            'ssh': 'a-oneterm-ssh2',
            'rdp': 'a-oneterm-ssh1',
            'vnc': 'oneterm-rdp',
            'telnet': 'a-telnet1',
            'redis': 'oneterm-redis',
            'mysql': 'oneterm-mysql',
            'mongodb': 'a-mongoDB1',
            'postgresql': 'a-postgreSQL1',
          }

          const tableData = res?.data?.list || []

          tableData.forEach((row) => {
            row._protocols = row?.protocols?.map((item) => {
              const key = item?.split?.(':')?.[0] || ''

              return {
                key,
                value: item,
                icon: protocolIconMap?.[key] || ''
              }
            }) || []

            const accountList = []
            row._protocols.forEach((protocol) => {
              Object.keys(row.authorization || {}).forEach((acc_id) => {
                const _find = this.accountList?.find((item) => Number(item.id) === Number(acc_id))
                if (_find) {
                  accountList.push({
                    account_id: _find.id,
                    account_name: _find.name,
                    protocol: protocol.value,
                    protocolType: protocol.key,
                    protocolIcon: protocol.icon,
                  })
                }
              })
            })
            row.accountList = accountList
          })

          this.tableData = tableData
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

    onCellClick(e) {
      const opsTable = this.$refs.opsTable.getVxetableRef()
      opsTable.toggleRowExpand(e.row)
    },

    clickProtocol(protocol, row) {
      const accountList = []

      Object.keys(row.authorization).forEach((acc_id) => {
        const _find = this.accountList?.find((item) => Number(item.id) === Number(acc_id))

        if (_find) {
          accountList.push({
            account_id: _find.id,
            account_name: _find.name,
          })
        }
      })

      if (accountList.length > 1) {
        this.$refs.loginModalRef.open(row.id, row.name, row.authorization, [protocol.value])
      } else if (accountList.length === 1) {
        this.$emit('openTerminal', {
          assetId: row.id,
          assetName: row.name,
          accountId: accountList[0].account_id,
          protocol: protocol.value,
          protocolType: protocol.key
        })
      }
    },

    openTerminal(assetId, assetName, data) {
      this.$emit('openTerminal', {
        assetId,
        assetName,
        accountId: data.account_id,
        protocol: data.protocol,
        protocolType: data.protocolType
      })
    },

    loginOpenTerminal(data) {
      this.$emit('openTerminal', data)
    },

    loginOpenTerminalList(data) {
      this.$emit('openTerminalList', data)
    }
  }
}
</script>

<style lang="less" scoped>
.workstation-asset-table {
  &-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
  }

  &-account {
    padding: 8px 0px 8px 16px;
    border-left: solid 3px @primary-color_9;
    border-top: 1px solid #E4E7ED;
    background-color: #F9FBFF;
    box-shadow: 0px -2px 6px 0px rgba(98, 147, 192, 0.10) inset, 0px 2px 6px 0px rgba(98, 147, 192, 0.10) inset;

    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 17px;

    &-protocol {
      font-size: 14px;
      color: #2F54EB;
      margin-right: 8px;
    }

    &-name {
      font-size: 14px;
      font-weight: 400;
      color: #1D2129;
    }

    &-item {
      padding: 0px 8px;
      background-color: #EBEFF8;
      height: 30px;
      display: flex;
      align-items: center;
      border-radius: 2px;
      border: 1px solid transparent;
      cursor: pointer;

      &:hover {
        border-color: #7F97FA;
        background-color: #E1EFFF;
      }
    }
  }

  &-status {
    border-radius: 22px;
    line-height: 24px;
    padding: 5px 15px 5px 25px;
    position: relative;

    &::after {
      content: '';
      position: absolute;
      width: 9px;
      height: 9px;
      border-radius: 50%;
      top: 50%;
      transform: translateY(-50%);
      left: 10px;
    }
  }

  &-right::after {
    background-color: #4dcb73;
  }

  &-error::after {
    background-color: #f2637b;
  }

  &-operation-btn {
    font-size: 15px;

    &:not(:first-child) {
      margin-left: 6px;
    }
  }

  &-pagination {
    text-align: right;
    margin-top: 8px;
  }
}
</style>
