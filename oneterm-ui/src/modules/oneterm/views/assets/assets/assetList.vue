<template>
  <TwoColumnLayout
    class="oneterm-asset-list"
    :appName="`oneterm-asset-list-${forMyAsset}`"
    :style="{ height: `${windowHeight - 112}px` }"
  >
    <template #one>
      <div class="asset-list-sidebar-header" v-if="!forMyAsset">
        <strong>{{ $t(`oneterm.assetList.assetTree`) }}</strong>
        <div
          @click="
            () => {
              $emit('openNode', null)
            }
          "
        >
          <span><a-icon type="plus" style="color:#2F54EB"/></span>
          {{ $t(`oneterm.assetList.createFloder`) }}
        </div>
      </div>
      <a-tree
        v-if="refreshTreeFlag"
        class="asset-list-sidebar-tree"
        :selectedKeys="selectedKeys"
        :load-data="onLoadData"
        :tree-data="treeData"
        :replaceFields="{
          children: 'children',
          title: 'name',
          key: 'id',
        }"
      >
        <template #title="node">
          <a-dropdown :trigger="['contextmenu']" :disabled="forMyAsset">
            <div class="asset-list-sidebar-tree-title" @click="clickNode(node.dataRef)">
              <ops-icon :type="selectedKeys[0] === node.dataRef.id ? 'oneterm-file-selected' : 'oneterm-file'" />
              <span :title="node.dataRef.name">{{ node.dataRef.name }}</span>
              <span>({{ node.dataRef.asset_count }})</span>
            </div>
            <template #overlay>
              <a-menu>
                <a-menu-item key="1" @click="$emit('openNode', { parent_id: node.dataRef.id })">{{
                  $t(`oneterm.assetList.createFloder`)
                }}</a-menu-item>
                <a-menu-item key="2" @click="$emit('openNode', node.dataRef)">{{
                  $t(`oneterm.assetList.editFloder`)
                }}</a-menu-item>
                <a-menu-item key="3" @click="deleteNode(node.dataRef)">{{
                  $t(`oneterm.assetList.deleteFloder`)
                }}</a-menu-item>
              </a-menu>
            </template>
          </a-dropdown>
        </template>
      </a-tree>
    </template>
    <template #two>
      <div class="oneterm-layout-container" :style="{ height: '100%' }">
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
          </a-space>
          <a-space>
            <div class="ops-list-batch-action" v-show="!!selectedRowKeys.length">
              <span @click="batchUpdate('access_auth')">{{ $t('oneterm.accessRestrictions') }}</span>
              <span @click="batchUpdate('account_ids')">{{ $t('grant') }}</span>
              <span @click="batchUpdate('protocols')">{{ $t('oneterm.assetList.editProtocol') }}</span>
              <span>{{ $t('selectRows', { rows: selectedRowKeys.length }) }}</span>
            </div>
            <a-button v-if="!forMyAsset && selectedKeys && selectedKeys.length" type="primary" @click="createAsset">{{
              $t(`create`)
            }}</a-button>
            <a-button
              @click="
                () => {
                  updateTableData()
                  selectedRowKeys = []
                  $refs.opsTable.getVxetableRef().clearCheckboxRow()
                  $refs.opsTable.getVxetableRef().clearCheckboxReserve()
                }
              "
            >{{ $t(`refresh`) }}</a-button
            >
          </a-space>
        </div>
        <div class="asset-list-table">
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
            height="auto"
            resizable
            :loading="loading"
          >
            <vxe-column type="checkbox" width="60px" v-if="!forMyAsset"></vxe-column>
            <vxe-column :title="$t(`oneterm.name`)" field="name"> </vxe-column>
            <vxe-column :title="$t(`oneterm.assetList.ip`)" field="ip"> </vxe-column>
            <vxe-column :title="$t(`oneterm.assetList.nodeName`)" field="node_chain"> </vxe-column>
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
                  <a @click="openAsset(row)"><ops-icon type="icon-xianxing-edit"/></a>
                  <a-popconfirm :title="$t('confirmDelete')" @confirm="deleteAsset(row)">
                    <a style="color:red"><ops-icon type="icon-xianxing-delete"/></a>
                  </a-popconfirm>
                </a-space>
                <a-tooltip v-else :title="$t(`login`)">
                  <a
                    :disabled="!Object.keys(row.authorization).length || !row.protocols.length"
                    @click="openLogin(row.id, row.authorization, row.protocols)"
                  ><ops-icon
                    type="oneterm-switch"
                  /></a>
                </a-tooltip>
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
      <LoginModal ref="loginModal" />
    </template>
  </TwoColumnLayout>
</template>

<script>
import _ from 'lodash'
import { mapState } from 'vuex'
import TwoColumnLayout from '@/components/TwoColumnLayout'
import { getNodeList, deleteNodeById, getNodeById } from '../../../api/node'
import { getAssetList, deleteAssetById } from '../../../api/asset'
import BatchUpdateModal from './batchUpdateModal.vue'
import LoginModal from './loginModal.vue'
export default {
  name: 'AssetList',
  components: { TwoColumnLayout, BatchUpdateModal, LoginModal },
  props: {
    forMyAsset: {
      type: Boolean,
      default: false,
    },
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
      loading: false,
      refreshTreeFlag: false,
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
        this.updateTableData()
      },
    },
  },
  mounted() {
    this.getFirstLayout()
  },
  methods: {
    getFirstLayout() {
      this.selectedKeys = []
      this.chainIds = ''
      this.treeData = []
      this.refreshTreeFlag = false
      getNodeList({ parent_id: 0 }).then((res) => {
        this.treeData = (res?.data?.list ?? []).map((item) => ({
          ...item,
          chainId: `${item.id}`,
          isLeaf: !item.has_child,
        }))
        this.refreshTreeFlag = true
      })
    },
    onLoadData(treeNode) {
      return new Promise((resolve) => {
        if (treeNode.dataRef.children) {
          resolve()
          return
        }
        getNodeList({ parent_id: treeNode.dataRef.id }).then((res) => {
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
      await getNodeList({ ids: this.chainIds.split('@').join(',') }).then((result1) => {
        res1 = result1?.data?.list ?? []
      })
      if (parent_id) {
        await getNodeList({ self_parent: parent_id }).then((result2) => {
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
  },
}
</script>

<style lang="less" scoped>
@import '../../../style/index.less';
.asset-list-sidebar-header {
  font-size: 14px;
  display: flex;
  justify-content: space-between;
  > strong {
    color: #000;
  }
  > div {
    &:hover {
      color: @primary-color;
    }
    cursor: pointer;
    > span {
      display: inline-flex;
      background-color: #d5e8fe;
      border-radius: 50%;
      width: 14px;
      height: 14px;
      margin-right: 2px;
      justify-content: center;
      align-items: center;
      > i {
        font-size: 12px;
      }
    }
  }
}
.asset-list-table {
  height: calc(100% - 48px - 32.5px);
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
    }
  }
}
</style>
