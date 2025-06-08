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
        info: true
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
    margin-bottom: 7px;
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
      font-weight: 700;
      color: #000000;
      flex-shrink: 0;
    }
  }

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
    }
  }
}
</style>
