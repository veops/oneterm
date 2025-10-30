<template>
  <TwoColumnLayout
    class="oneterm-asset-list"
    :appName="`oneterm-workstation-asset-list`"
    :style="{ height: `${windowHeight - 88}px` }"
    :triggerLength="8"
    calcBasedParent
  >
    <template #one>
      <div class="asset-list-sidebar">
        <div class="asset-list-sidebar-header">
          <div class="asset-list-sidebar-title">
            <div class="asset-list-sidebar-title-text">
              {{ $t('oneterm.assetList.assetTree') }}
            </div>

            <a @click="openWebSSH">
              <ops-icon class="asset-list-sidebar-ssh" type="a-oneterm-ssh2"/>
            </a>
          </div>

          <a-input
            v-model="searchValue"
            class="asset-list-sidebar-search"
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
            </div>
          </template>
        </a-tree>
        <div
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
      <slot name="two"></slot>
    </template>
  </TwoColumnLayout>
</template>

<script>
import _ from 'lodash'
import { mapState } from 'vuex'
import { getNodeList } from '@/modules/oneterm/api/node'

import TwoColumnLayout from '@/components/TwoColumnLayout'

export default {
  name: 'AssetList',
  components: { TwoColumnLayout },
  props: {
    userStat: {
      type: Object,
      default: () => {}
    },
    selectedKeys: {
      type: Array,
      default: () => []
    }
  },
  data() {
    return {
      chainIds: '',
      treeData: [],
      filterName: '',
      tableData: [],
      tablePage: {
        currentPage: 1,
        pageSize: 20,
        totalResult: 0,
      },
      loading: false,
      refreshTreeFlag: false,
      searchValue: '',

      getRequestParams: {
        info: false
      }
    }
  },
  computed: {
    ...mapState({
      windowHeight: (state) => state.windowHeight
    }),
    userStatList() {
      const userStatList = [
        {
          icon: 'oneterm-connect1',
          title: 'oneterm.connectedAssets',
          progress: 0,
          count: this.userStat?.asset || 0,
          allCount: this.userStat?.total_asset || 0
        },
        {
          icon: 'oneterm-session1',
          title: 'oneterm.connectedSession',
          progress: 0,
          count: this.userStat?.connect || 0,
          allCount: this.userStat?.session || 0
        }
      ]

      if (userStatList[0].count && userStatList[0].allCount) {
        userStatList[0].progress = Math.round(userStatList[0].count / userStatList[0].allCount * 100)
      }

      if (userStatList[1].count && userStatList[1].allCount) {
        userStatList[1].progress = Math.round(userStatList[1].count / userStatList[1].allCount * 100)
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
  async mounted() {
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

    onLoadData(treeNode) {
      return new Promise((resolve) => {
        if (treeNode.dataRef.children) {
          resolve()
          return
        }
        getNodeList({
          parent_id: treeNode.dataRef.id,
          info: this.getRequestParams.info
        }).then((res) => {
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
      if (this.selectedKeys[0] === node.id) {
        this.$emit('updateSelectedKeys', [])
        this.chainIds = ''
      } else {
        this.$emit('updateSelectedKeys', [node.id])
        this.chainIds = node.chainId
      }
    },

    openRecentSession() {
      this.$emit('openRecentSession')
    },

    openWebSSH() {
      this.$emit('openWebSSH')
    }
  },
}
</script>

<style lang="less" scoped>
.asset-list-sidebar {
  height: 100%;
  display: flex;
  flex-direction: column;

  &-header {
    flex-shrink: 0;
    margin-bottom: 12px;
  }

  &-title {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    column-gap: 12px;
    row-gap: 6px;

    &-text {
      padding-left: 12px;
      border-left: solid 3px @primary-color;
      font-size: 15px;
      font-weight: 600;
      color: @text-color_1;
      flex-shrink: 0;
    }
  }

  &-ssh {
    font-size: 16px;
    color: @text-color_2;
    transition: all 0.2s ease;

    &:hover {
      color: @primary-color;
      transform: scale(1.1);
    }
  }

  &-search {
    margin-top: 16px;
    height: 36px;

    /deep/ .ant-input {
      height: 36px;
      border-radius: 6px;
      border: 1px solid #e8eaed;
      background-color: #fafbfc;
      transition: all 0.2s ease;
      font-size: 14px;

      &::placeholder {
        color: @text-color_3;
      }

      &:hover {
        border-color: #c3cdd7;
        background-color: #fff;
      }

      &:focus {
        border-color: @primary-color;
        background-color: #fff;
        box-shadow: 0 0 0 3px fade(@primary-color, 8%);
      }
    }

    /deep/ .ant-input-prefix {
      color: @text-color_3;
      font-size: 14px;
    }
  }

  &-tree {
    height: 100%;
    overflow-y: auto;
    overflow-x: hidden;
    margin-top: 8px;
  }

  .user-stat {
    width: 100%;
    margin-top: 20px;
    padding: 18px;
    background: linear-gradient(135deg, #fafbfc 0%, #f5f7fa 100%);
    border: 1px solid #e8eaed;
    border-radius: 8px;
    flex-shrink: 0;
    box-shadow: 0 1px 4px rgba(0, 0, 0, 0.04);

    &-item {
      width: 100%;

      &:not(:last-child) {
        margin-bottom: 12px;
        padding-bottom: 12px;
        border-bottom: 1px solid #e8eaed;
      }
    }

    &-header {
      display: flex;
      align-items: center;
      margin-bottom: 10px;

      &-icon {
        font-size: 16px;
        color: @primary-color;
      }

      &-title {
        margin-left: 8px;
        font-size: 13px;
        font-weight: 600;
        color: @text-color_1;
      }
    }

    &-data {
      display: flex;
      align-items: center;
      justify-content: space-between;

      &-progress {
        flex: 1;
        height: 8px;
        border-radius: 4px;
        background-color: #e8eaed;
        overflow: hidden;
        box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.05);

        &-content {
          height: 100%;
          background: linear-gradient(90deg, @primary-color 0%, lighten(@primary-color, 8%) 100%);
          transition: width 0.4s cubic-bezier(0.4, 0, 0.2, 1);
          box-shadow: 0 0 8px fade(@primary-color, 30%);
        }
      }

      &-count {
        margin-left: 12px;
        font-size: 13px;
        font-weight: 500;
        color: @text-color_2;
        white-space: nowrap;

        &-bold {
          font-weight: 700;
          font-size: 14px;
          color: @primary-color;
        }
      }
    }
  }
}
</style>
<style lang="less">
.oneterm-asset-list.two-column-layout .two-column-layout-main {
  padding: 0;
}
.asset-list-sidebar-tree.ant-tree {
  li .ant-tree-switcher {
    width: 18px !important;
  }

  li .ant-tree-node-content-wrapper {
    transition: all 0.2s ease;
    border-radius: 4px;
    padding-left: 4px !important;
    padding-right: 2px !important;

    &:hover {
      background-color: fade(@primary-color, 5%);
    }

    &.ant-tree-node-selected {
      background-color: fade(@primary-color, 10%);
      font-weight: 500;
    }
  }

  li .ant-tree-node-content-wrapper {
    width: calc(100% - 18px);
    height: 30px;
    line-height: 30px;

    .asset-list-sidebar-tree-title {
      display: flex;
      align-items: center;
      height: 30px;

      i {
        font-size: 16px;
        flex-shrink: 0;
        margin-right: 4px;
      }

      span:nth-child(2) {
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        font-size: 14px;
      }

      &-count {
        font-size: 11px;
        font-weight: 500;
        color: @text-color_3;
        background-color: fade(@primary-color, 8%);
        padding: 0 6px;
        height: 18px;
        line-height: 18px;
        border-radius: 9px;
        margin-left: 4px;
        text-align: center;
        flex-shrink: 0;
      }
    }
  }
}
</style>
