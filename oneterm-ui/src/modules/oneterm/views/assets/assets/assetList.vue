<template>
  <TwoColumnLayout
    class="oneterm-asset-list"
    :appName="`oneterm-asset-list-${forMyAsset}`"
    :style="{ height: `${windowHeight - 88}px` }"
    :triggerLength="8"
    calcBasedParent
  >
    <template #one>
      <div class="asset-list-sidebar">
        <div class="asset-list-sidebar-header">
          <slot name="title"></slot>

          <a-input
            v-model="searchValue"
            class="asset-list-sidebar-header-search"
            :placeholder="$t('oneterm.assetList.assetSearchTip')"
          >
            <a-icon slot="prefix" type="search" />
          </a-input>
        </div>
        <a-tree
          v-if="refreshTreeFlag"
          class="asset-list-sidebar-tree"
          :selectedKeys="selectedKeys"
          :load-data="onLoadData"
          :tree-data="filterTreeData"
          :replaceFields="{
            children: 'children',
            title: 'name',
            key: 'id',
          }"
        >
          <template #title="node">
            <div class="asset-list-sidebar-tree-title" @click="clickNode(node.dataRef)">
              <ops-icon :type="selectedKeys[0] === node.dataRef.id ? 'oneterm-file-selected' : 'oneterm-file'" />
              <span :title="node.dataRef.name">{{ node.dataRef.name }}</span>
              <span class="asset-list-sidebar-tree-title-count">{{ node.dataRef.asset_count }}</span>
              <a-dropdown v-if="!forMyAsset && showNodeOperation(node.dataRef, ['write', 'delete', 'grant'])" :disabled="forMyAsset">
                <ops-icon class="asset-list-sidebar-tree-title-more" type="veops-more" />

                <template #overlay>
                  <a-menu>
                    <a-menu-item key="1" v-if="showNodeOperation(node.dataRef, ['write'])" @click="$emit('openNode', { parent_id: node.dataRef.id })">
                      <a-icon type="plus-circle" />
                      {{ $t(`oneterm.assetList.createSubCatalog`) }}
                    </a-menu-item>
                    <a-menu-item key="2" v-if="showNodeOperation(node.dataRef, ['write'])" @click="$emit('openNode', node.dataRef)">
                      <ops-icon type="icon-xianxing-edit" />
                      {{ $t(`oneterm.assetList.editCatalog`) }}
                    </a-menu-item>
                    <a-menu-item key="3" v-if="showNodeOperation(node.dataRef, ['delete'])" @click="deleteNode(node.dataRef)">
                      <ops-icon type="veops-delete" />
                      {{ $t(`oneterm.assetList.deleteCatalog`) }}
                    </a-menu-item>
                    <template v-if="showNodeOperation(node.dataRef, ['grant'])">
                      <a-divider style="margin: 4px 0" />
                      <a-menu-item key="4" @click="openGrantModal(node.dataRef)">
                        <a-icon type="user-add" />
                        {{ $t(`oneterm.assetList.grantCatalog`) }}
                      </a-menu-item>
                    </template>
                  </a-menu>
                </template>
              </a-dropdown>
            </div>
          </template>
        </a-tree>
        <div
          v-if="forMyAsset"
          class="user-stat"
        >
          <div
            v-for="(item, index) in userStatList"
            :key="index"
            class="user-stat-item"
          >
            <div class="user-stat-header">
              <ops-icon class="user-stat-header-icon" :type="item.icon" />
              <span class="user-stat-header-title">{{ $t(item.title) }}</span>
            </div>
            <div class="user-stat-data">
              <div class="user-stat-data-progress">
                <div
                  class="user-stat-data-progress-content"
                  :style="{
                    width: item.progress + '%'
                  }"
                ></div>
              </div>
              <div class="user-stat-data-count">
                <span class="user-stat-data-count-bold">{{ item.count }}</span>/{{ item.allCount }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </template>
    <template #two>
      <div class="oneterm-layout-container" :style="{ height: '100%' }">
        <!-- tab 切换 -->
        <slot name="two-tab"></slot>
        <div
          v-show="showAssetTable"
          :style="{
            height: forMyAsset ? 'calc(100% - 60px)' : '100%',
          }"
        >
          <div class="oneterm-layout-container-header">
            <a-space>
              <a-input-search
                allow-clear
                v-model="filterName"
                :style="{ width: '250px' }"
                :placeholder="$t('placeholderSearch')"
                @search="updateTableData()"
              />
            </a-space>
            <a-space>
              <div class="ops-list-batch-action" v-show="!!selectedRowKeys.length">
                <span @click="batchUpdate('access_auth')">{{ $t('oneterm.accessRestrictions') }}</span>
                <span @click="authAsset">{{ $t('grant') }}</span>
                <span @click="batchUpdate('protocols')">{{ $t('oneterm.assetList.editProtocol') }}</span>
                <span>{{ $t('selectRows', { rows: selectedRowKeys.length }) }}</span>
              </div>
              <a-button
                v-if="!forMyAsset && selectedKeys && selectedKeys.length && showCreateNodeBtn"
                type="primary"
                @click="createAsset"
              >
                {{ $t(`create`) }}
              </a-button>
              <a-button
                type="primary"
                class="ops-button-ghost"
                ghost
                @click="handleRefresh"
              >
                <ops-icon type="veops-refresh" />
                {{ $t('refresh') }}
              </a-button>
            </a-space>
          </div>
          <div class="asset-list-table">
            <ops-table
              size="small"
              ref="opsTable"
              :data="tableData"
              show-overflow
              show-header-overflow
              @checkbox-change="onSelectChange"
              @checkbox-all="onSelectChange"
              @checkbox-range-end="onSelectRangeEnd"
              :checkbox-config="{ reserve: true, highlight: true, range: true }"
              :expand-config="{iconOpen: 'vxe-icon-square-minus', iconClose: 'vxe-icon-square-plus'}"
              :row-config="{ keyField: 'id' }"
              height="auto"
              resizable
              :loading="loading"
            >
              <vxe-column type="checkbox" width="60px" v-if="!forMyAsset"></vxe-column>
              <vxe-column :type="forMyAsset ? 'expand' : ''" :title="$t(`oneterm.name`)" field="name">
                <template #default="{ row }">
                  <span>{{ row.name }}</span>
                </template>
                <template #content="{ row }">
                  <div v-if="row.accountList.length" class="oneterm-table-account">
                    <div
                      v-for="(item) in row.accountList"
                      :key="item.protocol + item.account_id"
                      class="oneterm-table-account-item"
                      @click="openTerminal(row.id, row.name, item)"
                    >
                      <ops-icon class="oneterm-table-account-protocol" :type="item.protocolIcon" />
                      <span class="oneterm-table-account-name">{{ item.account_name }}</span>
                    </div>
                  </div>
                </template>
              </vxe-column>
              <vxe-column :title="$t(`oneterm.assetList.ip`)" field="ip"> </vxe-column>
              <vxe-column :title="$t(`oneterm.assetList.catalogName`)" field="node_chain"> </vxe-column>
              <vxe-column
                :title="$t(`oneterm.assetList.connectable`)"
                field="connectable"
                align="center"
                min-width="105px"
              >
                <template #default="{row}">
                  <span class="oneterm-table-right" v-if="row.connectable">{{ $t(`oneterm.assetList.connected`) }}</span>
                  <span class="oneterm-table-right oneterm-table-error" v-else>{{ $t(`oneterm.assetList.error`) }}</span>
                </template>
              </vxe-column>
              <vxe-column :title="$t(`operation`)" :width="100" align="center">
                <template #default="{row}">
                  <a-space v-if="!forMyAsset">
                    <a
                      v-if="row.accountList.length && showAssetOperation(row, 'grant')"
                      @click="createTempLink(row)"
                    >
                      <ops-icon type="veops-link" />
                    </a>
                    <a
                      v-if="showAssetOperation(row, 'write')"
                      @click="openAsset(row)"
                    >
                      <ops-icon type="icon-xianxing-edit"/>
                    </a>
                    <a-popconfirm
                      v-if="showAssetOperation(row, 'delete')"
                      :title="$t('confirmDelete')"
                      @confirm="deleteAsset(row)"
                    >
                      <a style="color:red"><ops-icon type="icon-xianxing-delete"/></a>
                    </a-popconfirm>
                  </a-space>

                  <a-space v-else-if="row.accountList.length">
                    <a-tooltip
                      v-for="(item) in row._protocols"
                      :key="item.key"
                      :title="item.key"
                    >
                      <a
                        class="oneterm-table-operation-btn"
                        @click="clickProtocol(item, row)"
                      >
                        <ops-icon v-if="item.icon" :type="item.icon" />
                      </a>
                    </a-tooltip>
                  </a-space>
                </template>
              </vxe-column>
            </ops-table>
          </div>
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
      </div>
      <BatchUpdateModal
        ref="batchUpdateModal"
        :selectedRowKeys="selectedRowKeys"
        @submit="
          () => {
            updateTableData()
            selectedRowKeys = []
            $refs.opsTable.getVxetableRef().clearCheckboxRow()
            $refs.opsTable.getVxetableRef().clearCheckboxReserve()
          }
        "
      />
      <LoginModal
        ref="loginModal"
        :showProtocol="false"
        :choiceAccountByCheckbox="true"
        :forMyAsset="forMyAsset"
        @openTerminal="loginOpenTerminal"
        @openTerminalList="loginOpenTerminalList"
      />

      <TempLinkModal
        ref="tempLinkModalRef"
        :accountList="accountList"
      />

      <GrantModal ref="grantModalRef" />
    </template>
  </TwoColumnLayout>
</template>

<script>
import _ from 'lodash'
import { mapState } from 'vuex'
import TwoColumnLayout from '@/components/TwoColumnLayout'
import { getNodeList, deleteNodeById, getNodeById } from '../../../api/node'
import { getAssetList, deleteAssetById } from '../../../api/asset'
import { getAccountList } from '../../../api/account'
import BatchUpdateModal from './batchUpdateModal.vue'
import LoginModal from './loginModal.vue'
import TempLinkModal from './tempLink/tempLinkModal.vue'
import GrantModal from '@/modules/oneterm/components/grant/grantModal.vue'

export default {
  name: 'AssetList',
  components: { TwoColumnLayout, BatchUpdateModal, LoginModal, TempLinkModal, GrantModal },
  props: {
    forMyAsset: {
      type: Boolean,
      default: false,
    },
    userStat: {
      type: Object,
      default: () => {}
    },
    showAssetTable: {
      type: Boolean,
      default: true
    }
  },
  data() {
    return {
      selectedKeys: [],
      chainIds: '',
      treeData: [],
      filterName: '',
      tableData: [],
      tablePage: {
        currentPage: 1,
        pageSize: 20,
        totalResult: 0,
      },
      selectedRowKeys: [],
      accountList: [],
      loading: false,
      refreshTreeFlag: false,
      searchValue: '',
    }
  },
  computed: {
    ...mapState({
      windowHeight: (state) => state.windowHeight,
      rid: (state) => state.user.rid,
      roles: (state) => state.user.roles,
    }),
    isCreateRootNode() {
      return true
    },
    showCreateNodeBtn() {
      if (this.selectedKeys.length === 0) {
        return this.isCreateRootNode
      }
      let currentNode = null
      this.treeForeach(this.treeData, (node) => {
        if (node.id === this.selectedKeys[0]) {
          currentNode = node
        }
      })
      return currentNode?.permissions?.some?.((perm) => perm === 'write') || false
    },
    userStatList() {
      const userStatList = [
        {
          icon: 'oneterm-connect1',
          title: 'oneterm.connectedAssets',
          progress: 0,
          count: 0,
          allCount: 0
        },
        {
          icon: 'oneterm-session1',
          title: 'oneterm.connectedSession',
          progress: 0,
          count: 0,
          allCount: 0
        }
      ]

      if (this.forMyAsset) {
        userStatList[0].count = this.userStat?.asset || 0
        userStatList[0].allCount = this.userStat?.total_asset || 0

        if (userStatList[0].count && userStatList[0].allCount) {
          userStatList[0].progress = Math.round(userStatList[0].count / userStatList[0].allCount * 100)
        }

        userStatList[1].count = this.userStat?.connect || 0
        userStatList[1].allCount = this.userStat?.session || 0

        if (userStatList[1].count && userStatList[1].allCount) {
          userStatList[1].progress = Math.round(userStatList[1].count / userStatList[1].allCount * 100)
        }
      }

      return userStatList
    },
    filterTreeData() {
      if (this.searchValue) {
        let treeData = _.cloneDeep(this.treeData)
        treeData = treeData.filter((data) => {
          return this.handleTreeDataBySearch(data)
        })
        return treeData
      }
      return this.treeData
    }
  },
  watch: {
    selectedKeys: {
      immediate: true,
      deep: true,
      handler() {
        this.updateTableData()
      },
    },
  },
  async mounted() {
    await this.getAccountList()
    this.getFirstLayout()
  },
  methods: {
    handleTreeDataBySearch(data) {
      const isMatch = data?.name?.indexOf?.(this.searchValue) !== -1
      if (!data?.children?.length) {
        return isMatch ? data : null
      }

      data.children = data.children.filter((data) => {
        return this.handleTreeDataBySearch(data)
      })
      return isMatch || data.children.length ? data : null
    },

    getFirstLayout() {
      this.selectedKeys = []
      this.chainIds = ''
      this.treeData = []
      this.refreshTreeFlag = false
      getNodeList({
        info: this.forMyAsset,
        parent_id: 0
      }).then((res) => {
        this.treeData = (res?.data?.list ?? []).map((item) => ({
          ...item,
          chainId: `${item.id}`,
          isLeaf: !item.has_child,
        }))
        this.refreshTreeFlag = true
      })
    },

    async getAccountList() {
      const res = await getAccountList({ page_index: 1, info: this.forMyAsset })
      this.accountList = res?.data?.list || []
    },

    onLoadData(treeNode) {
      return new Promise((resolve) => {
        if (treeNode.dataRef.children) {
          resolve()
          return
        }
        getNodeList({ parent_id: treeNode.dataRef.id, info: this.forMyAsset }).then((res) => {
          treeNode.dataRef.children = (res?.data?.list ?? []).map((item) => ({
            ...item,
            chainId: `${treeNode.dataRef.chainId}@${item.id}`,
            isLeaf: !item.has_child,
          }))
          this.treeData = [...this.treeData]
          resolve()
        })
      })
    },
    clickNode(node) {
      this.selectedRowKeys = []
      this.$refs.opsTable.getVxetableRef().clearCheckboxRow()
      this.$refs.opsTable.getVxetableRef().clearCheckboxReserve()
      if (this.selectedKeys[0] === node.id) {
        this.selectedKeys = []
        this.chainIds = ''
      } else {
        this.selectedKeys = [node.id]
        this.chainIds = node.chainId
      }
    },
    deleteNode(node) {
      const that = this
      this.$confirm({
        title: that.$t('warning'),
        content: `${that.$t('confirmDelete2', { name: node.name })}`,
        onOk() {
          deleteNodeById(node.id).then((res) => {
            that.$message.success(that.$t('deleteSuccess'))
            that.getFirstLayout()
          })
        },
      })
    },
    updateTableData(currentPage = 1, pageSize = this.tablePage.pageSize) {
      this.loading = true
      getAssetList({
        parent_id: this.selectedKeys[0],
        page_index: currentPage,
        page_size: pageSize,
        search: this.filterName,
        info: this.forMyAsset,
      })
        .then(async (res) => {
          const protocolIconMap = {
            'ssh': 'a-oneterm-ssh2',
            'rdp': 'a-oneterm-ssh1',
            'vnc': 'oneterm-rdp',
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
          this.tablePage = {
            ...this.tablePage,
            currentPage,
            pageSize,
            totalResult: res?.data?.count ?? 0,
          }

          if (this.treeData && this.selectedKeys?.[0]) {
            const updateDataRes = await getNodeList({
              self_parent: this.selectedKeys?.[0],
              info: this.forMyAsset
            })
            const updateDataList = updateDataRes?.data?.list || []

            if (updateDataList.length) {
              this.treeForeach(this.treeData, (node) => {
                const updateData = updateDataList?.find?.((data) => node.id === data.id)
                if (updateData) {
                  this.$set(node, 'asset_count', updateData?.asset_count ?? 0)
                }
              })
            }
          }
        })
        .finally(() => {
          this.loading = false
        })
    },
    onSelectChange() {
      const opsTable = this.$refs.opsTable.getVxetableRef()
      const records = [...opsTable.getCheckboxRecords(), ...opsTable.getCheckboxReserveRecords()]
      this.selectedRowKeys = records
    },
    onSelectRangeEnd({ records }) {
      this.selectedRowKeys = records
    },
    pageOrSizeChange(currentPage, pageSize) {
      this.updateTableData(currentPage, pageSize)
    },
    openAsset(data) {
      this.$emit('openAsset', data)
    },
    deleteAsset(row) {
      this.loading = true
      deleteAssetById(row.id)
        .then((res) => {
          this.$message.success(this.$t('deleteSuccess'))
          this.updateTableData()
          this.updateNodeCount()
        })
        .finally(() => {
          this.loading = false
        })
    },
    batchUpdate(type) {
      this.$refs.batchUpdateModal.open(type)
    },
    treeForeach(tree, func) {
      tree.forEach((data) => {
        func(data)
        data.children && this.treeForeach(data.children, func)
      })
    },
    async updateNodeCount(parent_id = null) {
      let res1 = []
      let res2 = []
      await getNodeList({
        ids: this.chainIds.split('@').join(','),
        info: this.forMyAsset
      }).then((result1) => {
        res1 = result1?.data?.list ?? []
      })
      if (parent_id) {
        await getNodeList({
          self_parent: parent_id,
          info: this.forMyAsset
        }).then((result2) => {
          res2 = result2?.data?.list ?? []
        })
      }
      const data = _.uniqBy([...res1, ...res2], 'id')
      if (data.length) {
        this.treeForeach(this.treeData, (node) => {
          const _find = data.find((item) => item.id === node.id)
          if (_find) {
            this.$set(node, 'asset_count', _find.asset_count)
          }
        })
      }
    },
    openLogin(asset_id, authorization, protocols) {
      this.$refs.loginModal.open(asset_id, authorization, protocols)
    },
    createAsset() {
      getNodeById(this.selectedKeys[0]).then((res) => {
        if (res?.data?.list.length) {
          const node = res?.data?.list[0]
          const { protocols = [], authorization = {}, access_auth = {}, gateway_id = undefined } = node
          this.openAsset({ parent_id: this.selectedKeys[0], protocols, authorization, access_auth, gateway_id })
        }
      })
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
        this.$refs.loginModal.open(row.id, row.name, row.authorization, [protocol.value])
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

    loginOpenTerminal(data) {
      this.$emit('openTerminal', data)
    },

    loginOpenTerminalList(data) {
      this.$emit('openTerminalList', data)
    },

    async handleRefresh() {
      this.selectedRowKeys = []
      this.$refs.opsTable.getVxetableRef().clearCheckboxRow()
      this.$refs.opsTable.getVxetableRef().clearCheckboxReserve()
      await this.getAccountList()
      this.getFirstLayout()
    },

    createTempLink(row) {
      this.$refs.tempLinkModalRef.open(row)
    },

    openGrantModal(node) {
      this.$refs.grantModalRef.open({
        ids: [node.id],
        resourceId: node.resource_id,
        type: 'node'
      })
    },

    authAsset() {
      const assetIds = this.selectedRowKeys.map((item) => item.id)
      this.$refs.grantModalRef.open({
        resourceId: this.selectedRowKeys?.[0]?.resource_id || '',
        type: 'asset',
        ids: assetIds
      })
    },

    showNodeOperation(node, operationList) {
      const permissions = this?.roles?.permissions || []
      const isAdmin = permissions?.includes?.('oneterm_admin') || permissions?.includes?.('acl_admin')
      return node?.permissions?.some((item) => operationList.includes(item)) || isAdmin
    },

    showAssetOperation(asset, operaiton) {
      const permissions = this?.roles?.permissions || []
      const isAdmin = permissions?.includes?.('oneterm_admin') || permissions?.includes?.('acl_admin')
      return asset?.permissions?.some((item) => item === operaiton) || isAdmin
    }
  },
}
</script>

<style lang="less" scoped>
@import '../../../style/index.less';
.asset-list-sidebar {
  height: 100%;
  display: flex;
  flex-direction: column;

  &-header {
    flex-shrink: 0;
    margin-bottom: 7px;

    &-search {
      margin-top: 18px;
      height: 26px;
      line-height: 26px;

      /deep/ .ant-input {
        height: 26px;
        line-height: 26px;
        box-shadow: none;
      }
    }
  }

  &-tree {
    height: 100%;
    overflow-y: auto;
    overflow-x: hidden;
  }

  .user-stat {
    width: 100%;
    margin-top: 24px;
    padding-left: 34px;
    padding-right: 10px;
    flex-shrink: 0;

    &-item {
      width: 100%;
    }

    &-header {
      display: flex;
      align-items: center;

      &-icon {
        font-size: 14px;
      }

      &-title {
        margin-left: 7px;
        font-size: 14px;
        font-weight: 400;
        color: #4E5969;
      }
    }

    &-data {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-top: 6px;

      &-progress {
        width: 75%;
        height: 5px;
        border-radius: 5px;
        background-color: #EBEFF8;

        &-content {
          height: 100%;
          border-radius: 5px;
          background-color: #7F97FA;
        }
      }

      &-count {
        margin-left: 9px;
        font-size: 14px;
        font-weight: 500;
        color: #86909C;

        &-bold {
          font-weight: 500;
          color: #2F54EB;
        }
      }
    }
  }
}

.asset-list-table {
  height: calc(100% - 48px - 32.5px);
}

.oneterm-table-account {
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

.oneterm-table-operation-btn {
  font-size: 15px;

  &:not(:first-child) {
    margin-left: 6px;
  }
}
</style>
<style lang="less">
.oneterm-asset-list.two-column-layout .two-column-layout-main {
  padding: 0;
}
.asset-list-sidebar-tree.ant-tree {
  li .ant-tree-node-content-wrapper.ant-tree-node-selected {
    background-color: #e1efff;
  }
  li .ant-tree-node-content-wrapper {
    width: calc(100% - 24px);
    height: 30px;
    line-height: 30px;
    .asset-list-sidebar-tree-title {
      display: flex;
      align-items: center;
      padding-right: 5px;

      i {
        width: 24px;
      }
      span:nth-child(2) {
        flex: 1;
        width: 100%;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }

      &-count {
        font-size: 11px;
        font-weight: 400;
        color: #A5A9BC;
        margin-left: 12px;
      }

      &-more {
        display: none;
        margin-left: 4px;

        &:hover {
          color: #2f54eb;
        }
      }

      &:hover {
        .asset-list-sidebar-tree-title-more {
          display: inline-block;
        }
      }
    }
  }
}
</style>
