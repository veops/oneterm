<template>
  <div class="oneterm-layout">
    <AssetList
      ref="assetList"
      @openNode="openNode"
      @openAsset="openAsset"
    >
      <template #title>
        <div class="asset-list-title">
          <div class="asset-list-title-text">
            {{ $t('oneterm.assetList.assetList') }}
          </div>

          <div
            class="asset-list-title-create"
            @click="openNode()"
          >
            <span class="asset-list-title-create-icon">
              <a-icon type="plus"/>
            </span>
            <span class="asset-list-title-create-text">
              {{ $t(`oneterm.assetList.createCatalog`) }}
            </span>
          </div>
        </div>
      </template>
    </AssetList>
    <CreateNode ref="createNode" @submitNode="submitNode" />
    <CreateAsset ref="createAsset" @submitAsset="submitAsset" />
  </div>
</template>

<script>
import AssetList from './assetList.vue'
import CreateNode from './createNode.vue'
import CreateAsset from './createAsset.vue'
import { getAllDepAndEmployee } from '@/api/company'

export default {
  name: 'Assets',
  components: { AssetList, CreateNode, CreateAsset },
  provide() {
    return {
      provide_allTreeDepAndEmp: () => {
        return this.allTreeDepAndEmp
      },
    }
  },
  data() {
    return {
      type: 'create',
      allTreeDepAndEmp: [],
    }
  },
  computed: {
    title() {
      return this.$t(`oneterm.assetList.assetList`)
    },
  },
  mounted() {
    getAllDepAndEmployee({ block: 0 }).then((res) => {
      this.allTreeDepAndEmp = res
    })
  },
  methods: {
    openNode(node = null) {
      let type = 'create'
      if (node?.id) {
        type = 'edit'
      }
      this.$nextTick(() => {
        this.$refs.createNode.setNode(node, type)
      })
    },
    openAsset(asset) {
      let type = 'create'
      if (asset?.id) {
        type = 'edit'
      }
      this.$nextTick(() => {
        this.$refs.createAsset.setAsset(asset, type)
      })
    },
    submitNode() {
      this.$nextTick(() => {
        this.$refs.assetList.getFirstLayout()
      })
    },
    submitAsset(isUpdateNodeCount, parent_id = null) {
      this.$refs.assetList.updateTableData(this.$refs.assetList.tablePage.currentPage)
      if (isUpdateNodeCount) {
        this.$refs.assetList.updateNodeCount(parent_id)
      }
    },
  },
}
</script>

<style lang="less" scoped>
@import '../../../style/index.less';

.asset-list-title {
  height: 22px;
  line-height: 22px;
  padding-left: 12px;
  border-left: solid 3px @primary-color;
  display: flex;
  justify-content: space-between;
  align-items: center;

  &-text {
    font-size: 15px;
    font-weight: 700;
    color: #000000;
  }

  &-create {
    display: flex;
    align-items: center;
    cursor: pointer;
    font-size: 12px;

    &-icon {
      border-radius: 50%;
      width: 14px;
      height: 14px;
      margin-right: 2px;
      background-color: @primary-color_4;
      display: flex;
      align-items: center;
      justify-content: center;

      > i {
        font-size: 12px;
        color: @primary-color;
      }
    }

    &:hover {
      color: @primary-color;
    }
  }
}
</style>
