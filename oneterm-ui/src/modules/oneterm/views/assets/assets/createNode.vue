<template>
  <CustomDrawer
    :closable="true"
    :visible="visible"
    width="1000px"
    :title="title"
    @close="visible = false"
  >
    <p>
      <strong>{{ $t(`oneterm.baseInfo`) }}</strong>
    </p>
    <a-form-model
      ref="baseForm"
      :model="baseForm"
      :rules="baseRules"
      :label-col="{ span: 5 }"
      :wrapper-col="{ span: 16 }"
    >
      <a-form-model-item :label="$t('oneterm.assetList.folderName')" prop="name">
        <a-input v-model="baseForm.name" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
      <a-form-model-item :label="$t(`oneterm.folder`)" prop="parent_id">
        <treeselect
          class="custom-treeselect custom-treeselect-white"
          :style="{
            '--custom-height': '32px',
            lineHeight: '32px',
          }"
          v-model="baseForm.parent_id"
          :multiple="false"
          :clearable="true"
          searchable
          :options="nodeList"
          :placeholder="`${$t(`placeholder2`)}`"
          :normalizer="
            (node) => {
              return {
                id: node.id,
                label: node.name,
              }
            }
          "
        >
          <div
            :title="node.label"
            slot="option-label"
            slot-scope="{ node }"
            :style="{ width: '100%', whiteSpace: 'nowrap', textOverflow: 'ellipsis', overflow: 'hidden' }"
          >
            {{ node.label }}
          </div>
        </treeselect>
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.comment')" prop="comment">
        <a-textarea v-model="baseForm.comment" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
    </a-form-model>
    <p>
      <strong>{{ $t(`oneterm.protocol`) }}</strong>
    </p>
    <Protocol ref="protocol" />
    <p>
      <strong>{{ $t(`oneterm.accountAuthorization`) }}</strong>
    </p>
    <Account ref="account" />
    <div class="custom-drawer-bottom-action">
      <a-button
        :loading="loading"
        @click="
          () => {
            visible = false
          }
        "
      >{{ $t(`cancel`) }}</a-button
      >
      <a-button :loading="loading" @click="handleSubmit" type="primary">{{ $t(`confirm`) }}</a-button>
    </div>
  </CustomDrawer>
</template>

<script>
import { getNodeList, postNode, putNodeById } from '@/modules/oneterm/api/node'

import Protocol from './protocol.vue'
import Account from './account.vue'

export default {
  name: 'CreateNode',
  components: {
    Protocol,
    Account
  },
  data() {
    return {
      visible: false,
      type: 'create',
      nodeId: null,
      loading: false,
      baseForm: {
        name: '',
        parent_id: undefined,
        comment: '',
      },
      baseRules: {
        name: [{ required: true, message: `${this.$t(`placeholder1`)}` }],
      },
      nodeList: [],
    }
  },
  computed: {
    title() {
      if (this.type === 'create') {
        return this.$t(`oneterm.assetList.createFolder`)
      }
      return this.$t(`oneterm.assetList.editFolder`)
    },
  },
  mounted() {},
  methods: {
    setNode(node, type) {
      this.visible = true
      this.type = type
      this.$nextTick(async () => {
        const params = {}
        if (node?.id) {
          params.no_self_child = node.id
        }
        getNodeList(params).then((res) => {
          this.nodeList = res?.data?.list || []
        })

        const {
          id = null,
          name = '',
          comment = '',
          parent_id,
          gateway_id = undefined,
          protocols = [],
          authorization = {}
        } = node ?? {}

        this.nodeId = id
        this.baseForm = {
          name,
          parent_id: parent_id || undefined,
          comment,
        }
        this.$refs.protocol.setValues({ gateway_id, protocols })
        this.$refs.account.setValues({ authorization })
      })
    },
    handleSubmit() {
      this.$refs.baseForm.validate((valid) => {
        if (valid) {
          const { name, parent_id, comment } = this.baseForm
          const { gateway_id, protocols } = this.$refs.protocol.getValues()
          const { authorization } = this.$refs.account.getValues()
          const params = {
            name,
            comment,
            parent_id: parent_id ?? 0,
            protocols,
            gateway_id,
            authorization
          }

          this.loading = true
          if (this.nodeId) {
            putNodeById(this.nodeId, { ...params, id: this.nodeId })
              .then((res) => {
                this.$message.success(this.$t('editSuccess'))
                this.$emit('submitNode')
                this.visible = false
              })
              .finally(() => {
                this.loading = false
              })
          } else {
            postNode(params)
              .then((res) => {
                this.$message.success(this.$t('createSuccess'))
                this.$emit('submitNode')
                this.visible = false
              })
              .finally(() => {
                this.loading = false
              })
          }
        }
      })
    },
  },
}
</script>

<style lang="less" scoped>
.cmdb-radio-slot-field {
  display: flex;
  align-items: center;
  .slot-field1 {
    position: relative;
    > span {
      color: red;
      position: absolute;
      left: 72px;
      z-index: 1;
    }
    &::before {
      content: '';
      position: absolute;
      width: 10px;
      height: 10px;
      background-color: #e1efff;
      border-radius: 50%;
      right: -20px;
      top: 50%;
      transform: translateY(-50%);
    }
    &::after {
      content: '';
      position: absolute;
      width: 4px;
      height: 4px;
      background-color: #2f54eb;
      border-radius: 50%;
      right: -17px;
      top: 50%;
      transform: translateY(-50%);
    }
  }
  .slot-field2 {
    position: relative;
    &::before {
      content: '';
      position: absolute;
      width: 120px;
      height: 1px;
      background-color: #cacdd9;
      left: -130px;
      top: 50%;
      transform: translateY(-50%);
    }
    &::after {
      content: '';
      position: absolute;
      width: 0;
      height: 0;
      border-width: 5px;
      border-style: solid;
      border-color: transparent transparent transparent #cacdd9;
      left: -15px;
      top: 50%;
      transform: translateY(-50%);
    }
  }
}
</style>
<style lang="less">
.asset-create-node-container {
  .ant-form-item {
    margin-bottom: 8px;
  }
}

.cmdb-value-filter {
  .ant-form-item-control {
    line-height: 24px;
  }
  .table-filter-add {
    line-height: 40px;
  }
}

.cmdb-radio-slot-field {
  .ant-input[disabled] {
    background-color: #fff;
    cursor: default;
    color: rgba(0, 0, 0, 0.65);
    padding-left: 80px;
  }
}
</style>
