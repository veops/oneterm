<template>
  <TwoColumnLayout
    class="oneterm-asset-list"
    appName="oneterm-asset-list-management"
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
              <a-dropdown v-if="showNodeOperation(node.dataRef, ['write', 'delete', 'grant'])">
                <ops-icon class="asset-list-sidebar-tree-title-more" type="veops-more" />

                <template #overlay>
                  <a-menu>
                    <a-menu-item key="1" v-if="showNodeOperation(node.dataRef, ['write'])" @click="$emit('openNode', { parent_id: node.dataRef.id })">
                      <a-icon type="plus-circle" />
                      {{ $t(`oneterm.assetList.createSubFolder`) }}
                    </a-menu-item>
                    <a-menu-item key="2" v-if="showNodeOperation(node.dataRef, ['write'])" @click="$emit('openNode', node.dataRef)">
                      <ops-icon type="icon-xianxing-edit" />
                      {{ $t(`oneterm.assetList.editFolder`) }}
                    </a-menu-item>
                    <a-menu-item key="3" v-if="showNodeOperation(node.dataRef, ['delete'])" @click="deleteNode(node.dataRef)">
                      <ops-icon type="veops-delete" />
                      {{ $t(`oneterm.assetList.deleteFolder`) }}
                    </a-menu-item>
                    <template v-if="showNodeOperation(node.dataRef, ['grant'])">
                      <a-divider style="margin: 4px 0" />
                      <a-menu-item key="4" @click="openGrantModal(node.dataRef)">
                        <a-icon type="user-add" />
                        {{ $t(`oneterm.assetList.grantFolder`) }}
                      </a-menu-item>
                    </template>
                  </a-menu>
                </template>
              </a-dropdown>
            </div>
          </template>
        </a-tree>
      </div>
    </template>
    <template #two>
      <div class="oneterm-layout-container" :style="{ height: '100%' }">
        <div :style="{ height: '100%' }">
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
                v-if="selectedKeys && selectedKeys.length && showCreateNodeBtn"
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
              show-overflow
              show-header-overflow
              height="auto"
              resizable
              :data="tableData"
              :loading="loading"
              :checkbox-config="{ reserve: true, highlight: true, range: true }"
              :row-config="{ keyField: 'id', isHover: true }"
              @checkbox-change="onSelectChange"
              @checkbox-all="onSelectChange"
              @checkbox-range-end="onSelectRangeEnd"
            >
              <vxe-column type="checkbox" width="60px"></vxe-column>
              <vxe-column :title="$t(`oneterm.name`)" field="name"></vxe-column>
              <vxe-column :title="$t(`oneterm.assetList.ip`)" field="ip"> </vxe-column>
              <vxe-column :title="$t(`oneterm.assetList.folderName`)" field="node_chain"> </vxe-column>
              <vxe-column
                :title="$t(`status`)"
                field="connectable"
                align="center"
                min-width="105px"
              >
                <template #default="{row}">
                  <span class="oneterm-table-right" v-if="row.connectable">{{ $t(`oneterm.assetList.online`) }}</span>
                  <span class="oneterm-table-right oneterm-table-error" v-else>{{ $t(`oneterm.assetList.offline`) }}</span>
                </template>
              </vxe-column>
              <vxe-column :title="$t(`operation`)" :width="100" align="center">
                <template #default="{row}">
                  <a-space>
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
import { getNodeList, deleteNodeById, getNodeById } from '@/modules/oneterm/api/node'
import { getAssetList, deleteAssetById } from '@/modules/oneterm/api/asset'
import { getAccountList } from '@/modules/oneterm/api/account'
import { PROTOCOL_ICON } from './protocol/constants'

import BatchUpdateModal from './batchUpdateModal.vue'
import TempLinkModal from './tempLink/tempLinkModal.vue'
import GrantModal from '@/modules/oneterm/components/grant/grantModal.vue'
import TwoColumnLayout from '@/components/TwoColumnLayout'

export default {
  name: 'AssetList',
  components: { TwoColumnLayout, BatchUpdateModal, TempLinkModal, GrantModal },
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

      getRequestParams: {
        info: true
      }
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
        info: this.getRequestParams.info,
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
      const res = await getAccountList({ page_index: 1, info: this.getRequestParams.info })
      this.accountList = res?.data?.list || []
    },

    onLoadData(treeNode) {
      return new Promise((resolve) => {
        if (treeNode.dataRef.children) {
          resolve()
          return
        }
        getNodeList({ parent_id: treeNode.dataRef.id, info: this.getRequestParams.info }).then((res) => {
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
        info: this.getRequestParams.info,
      })
        .then(async (res) => {
          const tableData = res?.data?.list || []
          tableData.forEach((row) => {
            row._protocols = row?.protocols?.map((item) => {
              const key = item?.split?.(':')?.[0] || ''

              return {
                key,
                value: item,
                icon: PROTOCOL_ICON?.[key] || ''
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
              info: this.getRequestParams.info
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
        info: this.getRequestParams.info
      }).then((result1) => {
        res1 = result1?.data?.list ?? []
      })
      if (parent_id) {
        await getNodeList({
          self_parent: parent_id,
          info: this.getRequestParams.info
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
    createAsset() {
      getNodeById(this.selectedKeys[0]).then((res) => {
        if (res?.data?.list.length) {
          const node = res?.data?.list[0]
          const { protocols = [], authorization = {}, access_auth = {}, gateway_id = undefined } = node
          this.openAsset({ parent_id: this.selectedKeys[0], protocols, authorization, access_auth, gateway_id })
        }
      })
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
        resourceId: this.selectedRowKeys?.[0]?.resource_id ?? '',
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
