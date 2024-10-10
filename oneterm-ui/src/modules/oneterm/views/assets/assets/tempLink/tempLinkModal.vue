<template>
  <a-modal
    :visible="visible"
    :footer="null"
    :width="800"
    @cancel="handleCancel"
  >
    <a-tabs
      v-model="tabActive"
      @change="changeTab"
    >
      <a-tab-pane key="1" :tab="$t('oneterm.assetList.createTempLink')">
        <CreateTempLink
          v-if="assetData"
          :assetData="assetData"
          @cancel="handleCancel"
          @ok="handleCreateOk"
        />
      </a-tab-pane>
      <a-tab-pane key="2" :tab="$t('oneterm.assetList.tempLinkList')">
        <TempLinkTable
          v-if="assetData"
          ref="tempLinkTableRef"
          :assetData="assetData"
          :accountList="accountList"
        />
      </a-tab-pane>
    </a-tabs>
  </a-modal>
</template>

<script>
import CreateTempLink from './createTempLink.vue'
import TempLinkTable from './tempLinkTable.vue'

export default {
  name: 'TempLinkModal',
  components: {
    CreateTempLink,
    TempLinkTable
  },
  props: {
    accountList: {
      type: Array,
      default: () => []
    }
  },
  data() {
    return {
      visible: false,
      assetData: null,
      tabActive: '1'
    }
  },
  methods: {
    open(row) {
      this.visible = true
      this.assetData = row || null
    },
    handleCancel() {
      this.tabActive = '1'
      this.visible = false
      this.assetData = null
    },
    handleCreateOk() {
      this.tabActive = '2'
      this.$nextTick(() => {
        this.$refs.tempLinkTableRef.getTableData()
      })
    },
    changeTab(v) {
      if (v === '2') {
        this.$nextTick(() => {
          this.$refs.tempLinkTableRef.refreshTable()
        })
      }
    }
  }
}
</script>

<style lang="less" scoped>
</style>
