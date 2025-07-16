<template>
  <div>
    <a-form-model-item :label="$t('oneterm.auth.folderSelect')" prop="node_selector">
      <TypeRadio
        idName="oneterm.folder"
        :selectData="form.node_selector"
        @change="(value) => handleFormChange(['node_selector'], value)"
      >
        <template #id>
          <a-tree-select
            :value="form.node_selector.values"
            multiple
            style="width: 100%"
            :dropdown-style="{ maxHeight: '400px', overflow: 'auto' }"
            :placeholder="$t('oneterm.auth.selectItemTip', { name: $t('oneterm.folder') })"
            :tree-data="nodeSelectTreeData"
            :load-data="onLoadData"
            @change="(value) => handleFormChange(['node_selector', 'values'], value)"
          />
        </template>
      </TypeRadio>

      <div class="exclude-row">
        <div class="exclude-row-label">{{ $t('oneterm.auth.excludeItem', { name: $t('oneterm.folder') }) }}: </div>
        <a-tree-select
          :value="form.node_selector.exclude_ids"
          multiple
          style="width: 100%"
          :dropdown-style="{ maxHeight: '400px', overflow: 'auto' }"
          :placeholder="$t('oneterm.auth.excludeItemTip', { name: $t('oneterm.folder')})"
          :tree-data="nodeSelectTreeData"
          :load-data="onLoadData"
          @change="(value) => handleFormChange(['node_selector', 'exclude_ids'], value)"
        />
      </div>
    </a-form-model-item>

    <a-form-model-item :label="$t('oneterm.auth.assetSelect')" prop="asset_selector">
      <TypeRadio
        idName="oneterm.asset"
        :selectData="form.asset_selector"
        @change="(value) => handleFormChange(['asset_selector'], value)"
      >
        <template #id>
          <a-select
            mode="multiple"
            :value="form.asset_selector.values"
            :options="assetSelectOptions"
            :placeholder="$t('oneterm.auth.selectItemTip', { name: $t('oneterm.asset') })"
            @change="(value) => handleFormChange(['asset_selector', 'values'], value)"
          />
        </template>
      </TypeRadio>

      <div class="exclude-row">
        <div class="exclude-row-label">{{ $t('oneterm.auth.excludeItem', { name: $t('oneterm.asset')}) }}: </div>
        <a-select
          mode="multiple"
          :value="form.asset_selector.exclude_ids"
          :options="assetSelectOptions"
          :placeholder="$t('oneterm.auth.excludeItemTip', { name: $t('oneterm.asset')})"
          @change="(value) => handleFormChange(['asset_selector', 'exclude_ids'], value)"
        />
      </div>
    </a-form-model-item>
    <a-form-model-item :label="$t('oneterm.auth.accountSelect')" prop="account_selector">
      <TypeRadio
        idName="oneterm.account"
        :selectData="form.account_selector"
        @change="(value) => handleFormChange(['account_selector'], value)"
      >
        <template #id>
          <a-select
            mode="multiple"
            :value="form.account_selector.values"
            :options="accountSelectOptions"
            :placeholder="$t('oneterm.auth.selectItemTip', { name: $t('oneterm.account') })"
            @change="(value) => handleFormChange(['account_selector', 'values'], value)"
          />
        </template>
      </TypeRadio>
      <div class="exclude-row">
        <div class="exclude-row-label">{{ $t('oneterm.auth.excludeItem', { name: $t('oneterm.account')}) }}: </div>
        <a-select
          mode="multiple"
          :value="form.account_selector.exclude_ids"
          :options="accountSelectOptions"
          :placeholder="$t('oneterm.auth.excludeItemTip', { name: $t('oneterm.account')})"
          @change="(value) => handleFormChange(['account_selector', 'exclude_ids'], value)"
        />
      </div>
    </a-form-model-item>
  </div>
</template>

<script>
import { getAccountList } from '@/modules/oneterm/api/account'
import { getAssetList } from '@/modules/oneterm/api/asset'
import { getNodeList } from '@/modules/oneterm/api/node'

import TypeRadio from './typeRadio.vue'

export default {
  name: 'TargetSelect',
  components: {
    TypeRadio
  },
  props: {
    form: {
      type: Object,
      default: () => {}
    }
  },
  data() {
    return {
      nodeSelectTreeData: [],
      accountSelectOptions: [],
      assetSelectOptions: []
    }
  },
  mounted() {
    this.getNodeList()
    this.getAccountList()
    this.getAssetList()
  },
  methods: {
    getNodeList() {
      getNodeList({
        info: true,
        parent_id: 0
      }).then((res) => {
        const list = res?.data?.list || []
        this.nodeSelectTreeData = list.map((item) => ({
          id: String(item.id),
          value: String(item.id),
          label: item.name,
          isLeaf: !item.has_child,
        }))
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
          info: true
        }).then((res) => {
          treeNode.dataRef.children = (res?.data?.list ?? []).map((item) => ({
            id: String(item.id),
            value: String(item.id),
            label: item.name,
            isLeaf: !item.has_child,
          }))
          this.nodeSelectTreeData = [...this.nodeSelectTreeData]
          resolve()
        })
      })
    },
    getAccountList() {
      getAccountList({
        page_index: 1,
        page_size: 9999
      }).then((res) => {
        const list = res?.data?.list || []
        this.accountSelectOptions = list.map((item) => ({
          value: String(item.id),
          label: item.name
        }))
      })
    },
    getAssetList() {
      getAssetList({
        page_index: 1,
        page_size: 9999,
        info: true
      }).then((res) => {
        const list = res?.data?.list || []
        this.assetSelectOptions = list.map((item) => ({
          value: String(item.id),
          label: item.name
        }))
      })
    },
    handleFormChange(keys, value) {
      this.$emit(
        'change',
        {
          keys,
          value
        }
      )
    }
  }
}
</script>

<style lang="less" scoped>
.exclude-row {
  display: flex;
  align-items: flex-start;
  margin-top: 12px;

  &-label {
    line-height: 32px;
    flex-shrink: 0;
    margin-right: 12px;
  }
}
</style>
