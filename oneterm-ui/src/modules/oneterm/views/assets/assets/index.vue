<template>
  <div class="oneterm-layout">
    <div class="oneterm-header">{{ title }}</div>
    <AssetList ref="assetList" @openNode="openNode" @openAsset="openAsset" />
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
</style>
