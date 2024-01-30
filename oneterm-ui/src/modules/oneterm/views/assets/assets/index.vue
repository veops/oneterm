<template>
  <div class="oneterm-layout">
    <div class="oneterm-header">{{ title }}</div>
    <AssetList ref="assetList" v-show="pageType === 'list'" @openNode="openNode" @openAsset="openAsset" />
    <CreateNode
      ref="createNode"
      v-if="pageType === 'node'"
      @goBack="
        () => {
          pageType = 'list'
        }
      "
      @submitNode="submitNode"
    />
    <CreateAsset
      ref="createAsset"
      v-if="pageType === 'asset'"
      @goBack="
        () => {
          pageType = 'list'
        }
      "
      @submitAsset="submitAsset"
    />
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
      pageType: 'list', // list node assset
      type: 'create',
      allTreeDepAndEmp: [],
    }
  },
  computed: {
    title() {
      switch (this.pageType) {
        case 'node':
          if (this.type === 'create') {
            return this.$t(`oneterm.assetList.createFloder`)
          }
          return this.$t(`oneterm.assetList.editFloder`)
        case 'asset':
          if (this.type === 'create') {
            return this.$t(`oneterm.assetList.createAsset`)
          }
          return this.$t(`oneterm.assetList.editAsset`)
        default:
          return this.$t(`oneterm.assetList.assetList`)
      }
    },
  },
  mounted() {
    getAllDepAndEmployee({ block: 0 }).then((res) => {
      this.allTreeDepAndEmp = res
    })
  },
  methods: {
    openNode(node = null) {
      this.pageType = 'node'
      this.type = 'create'
      if (node?.id) {
        this.type = 'edit'
      }
      this.$nextTick(() => {
        this.$refs.createNode.setNode(node)
      })
    },
    openAsset(asset) {
      this.pageType = 'asset'
      this.type = 'create'
      if (asset?.id) {
        this.type = 'edit'
      }
      this.$nextTick(() => {
        this.$refs.createAsset.setAsset(asset)
      })
    },
    submitNode() {
      this.pageType = 'list'
      this.$nextTick(() => {
        this.$refs.assetList.getFirstLayout()
      })
    },
    submitAsset(isUpdateNodeCount, parent_id = null) {
      this.pageType = 'list'
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
</style>
