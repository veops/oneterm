<template>
  <a-modal
    :title="$t('oneterm.workStation.chooseAssets')"
    :visible="visible"
    :width="600"
    @cancel="handleCancel"
    @ok="handleOk"
  >
    <a-form-model
      ref="chooseAssetsFormRef"
      :model="form"
      :rules="rules"
      :label-col="{ span: 5 }"
      :wrapper-col="{ span: 16 }"
    >
      <a-form-model-item :label="$t('oneterm.protocol')" prop="protocol">
        <a-select
          :value="form.protocol"
          @change="handleProtocolChange"
        >
          <a-select-opt-group
            v-for="(protocolGroup, protocolGroupIndex) in protocolSelectOption"
            :key="protocolGroupIndex"
          >
            <div slot="label">{{ $t(protocolGroup.title) }}</div>
            <a-select-option
              v-for="(protocol) in protocolGroup.list"
              :key="protocol.key"
              :value="protocol.key"
            >
              <div class="protocol-select-item">
                <ops-icon :type="protocol.icon" />
                <span class="protocol-select-item-text">{{ protocol.label }}</span>
              </div>
            </a-select-option>
          </a-select-opt-group>
        </a-select>
      </a-form-model-item>
      <a-form-model-item
        v-if="visible"
        :label="$t('oneterm.workStation.chooseAssets')"
        prop="chooseConnect"
        class="choose-assets-tree"
      >
        <a-tree
          v-model="form.chooseConnect"
          :load-data="onLoadData"
          :tree-data="filterTreeData"
          checkable
          :selectable="false"
        >
          <template #title="node">
            <div class="choose-assets-tree-node">
              <ops-icon
                :type="node.dataRef.icon"
                class="choose-assets-tree-node-icon"
              />
              <span class="choose-assets-tree-node-title">
                {{ node.dataRef.title }}
                <span
                  v-if="node.dataRef.isConnect"
                  class="choose-assets-tree-node-title-account"
                >
                  ({{ node.dataRef.accountName }})
                </span>
              </span>
            </div>
          </template>
        </a-tree>
      </a-form-model-item>
    </a-form-model>
  </a-modal>
</template>

<script>
import _ from 'lodash'
import { getNodeList } from '@/modules/oneterm/api/node'
import { getAssetList } from '@/modules/oneterm/api/asset'
import { PROTOCOL_ICON, protocolSelectOption } from '@/modules/oneterm/views/assets/assets/protocol/constants'

export default {
  name: 'ChooseAssetsModal',
  props: {
    accountList: {
      type: Array,
      default: () => []
    }
  },
  data() {
    return {
      visible: false,
      treeData: [],
      allConnectMap: {},
      form: {
        protocol: 'ssh',
        chooseConnect: []
      },
      rules: {
        protocol: [{ required: true, message: this.$t('placeholder2') }],
        chooseConnect: [{ required: true, message: this.$t('placeholder2') }],
      },
      protocolSelectOption: _.cloneDeep(protocolSelectOption),
      getRequestParams: {
        info: true
      }
    }
  },
  computed: {
    filterTreeData() {
      if (this.form.protocol) {
        let treeData = _.cloneDeep(this.treeData)
        treeData = treeData.filter((data) => {
          return this.handleTreeDataBySearch(data)
        })

        return treeData
      }
      return this.treeData
    },
  },
  methods: {
    open() {
      this.visible = true
      this.initTreeData()
    },

    handleTreeDataBySearch(node) {
      if (node?.isConnect) {
        return node.protocolType === this.form.protocol ? node : null
      }

      if (node?.children?.length) {
        node.children = node.children.filter((node) => {
          return this.handleTreeDataBySearch(node)
        })
      }

      return node
    },

    async initTreeData() {
      const { nodeList, connectList } = await this.getNodeData(0)
      this.treeData = [
        ...nodeList,
        ...connectList
      ]

      const connectMap = {}
      connectList.map((connect) => {
        connectMap[connect.key] = connect
      })
      Object.assign(this.allConnectMap, connectMap)
    },

    async getNodeData(parentId) {
      const nodeRes = await getNodeList({
        info: this.getRequestParams.info,
        parent_id: parentId
      })
      const nodeList = nodeRes?.data?.list ?? []
      nodeList.map((item) => {
        item.isLeaf = false
        item.isConnect = false
        item.checkable = false
        item.key = `node-${item.id}`
        item.title = item.name
        item.icon = 'oneterm-file'
      })

      const connectList = []
      if (parentId !== 0) {
        const assetRes = await getAssetList({
          parent_id: parentId,
          page_index: 1,
          page_size: 9999,
          info: this.getRequestParams.info,
        })

        const assetList = assetRes?.data?.list || []
        assetList.map((asset) => {
          const protocols = asset?.protocols?.map((item) => {
            const key = item?.split?.(':')?.[0] || ''

            return {
              key,
              value: item,
              icon: PROTOCOL_ICON?.[key] || ''
            }
          }) || []

          protocols.forEach((protocol) => {
            Object.keys(asset.authorization || {}).forEach((acc_id) => {
              const _find = this.accountList?.find((item) => Number(item.id) === Number(acc_id))
              if (_find) {
                connectList.push({
                  isLeaf: true,
                  isConnect: true,
                  checkable: true,
                  assetId: asset.id,
                  key: `account-${asset.id}-${protocol.value}-${_find.id}`,
                  title: asset.name,
                  icon: protocol.icon,
                  protocol: protocol.value,
                  protocolType: protocol.key,
                  accountId: _find.id,
                  accountName: _find.name,
                })
              }
            })
          })
        })
      }

      return {
        nodeList,
        connectList
      }
    },

    async onLoadData(treeNode) {
      const { nodeList, connectList } = await this.getNodeData(treeNode.dataRef.id)
      const children = [
        ...nodeList,
        ...connectList
      ]

      this.treeForeach(this.treeData, (node) => {
        if (node.key === treeNode.dataRef.key) {
          node.children = children
        }
      })

      const connectMap = {}
      connectList.map((connect) => {
        connectMap[connect.key] = connect
      })
      Object.assign(this.allConnectMap, connectMap)
    },

    treeForeach(tree, func) {
      tree.forEach((data) => {
        func(data)
        data.children && this.treeForeach(data.children, func)
      })
    },

    handleProtocolChange(protocol) {
      if (protocol !== this.form.protocol) {
        this.form.protocol = protocol
        const chooseConnect = this.form.chooseConnect.filter((key) => {
          return this?.allConnectMap?.[key]?.protocolType === protocol
        })
        this.form.chooseConnect = chooseConnect
      }
    },

    handleCancel() {
      this.visible = false
      this.form = {
        protocol: 'ssh',
        chooseConnect: []
      }
      this.treeData = []
      this.allConnectMap = {}
      this.$refs.chooseAssetsFormRef.resetFields()
    },
    handleOk() {
      this.$refs.chooseAssetsFormRef.validate(async (valid) => {
        if (valid) {
          const batchExecutionData = this.form.chooseConnect.map((key) => {
            return this.allConnectMap?.[key] || {}
          })

          this.$emit('ok', batchExecutionData)
          this.handleCancel()
        }
      })
    }
  }
}
</script>

<style lang="less" scoped>
/deep/ .protocol-select-item {
  display: flex;
  align-items: center;

  &-text {
    margin-left: 6px;
  }
}

.choose-assets-tree {
  /deep/ .ant-tree {
    max-height: 40vh;
    overflow: auto;
  }

  &-node {
    display: flex;
    align-items: center;

    &-icon {
      margin-left: -3px;
    }

    &-title {
      margin-left: 4px;

      &-account {
        font-size: 12px;
        color: #A5A9BC;
      }
    }
  }
}
</style>
