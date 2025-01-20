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
      <a-form-model-item :label="$t(`oneterm.name`)" prop="name">
        <a-input v-model="baseForm.name" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
      <a-form-model-item label="IP" prop="ip">
        <a-input v-model="baseForm.ip" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
      <a-form-model-item :label="$t(`oneterm.catalog`)" prop="parent_id">
        <treeselect
          class="custom-treeselect custom-treeselect-white"
          :style="{
            '--custom-height': '32px',
            lineHeight: '32px'
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
                children: node.children && node.children.length ? node.children : undefined
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
    <p>
      <strong>{{ $t(`oneterm.accessRestrictions`) }}</strong>
    </p>
    <AccessAuth ref="accessAuth" />
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
import Protocol from './protocol.vue'
import Account from './account.vue'
import AccessAuth from './accessAuth.vue'
import { getNodeList } from '../../../api/node'
import { postAsset, putAssetById } from '../../../api/asset'

export default {
  name: 'CreateAsset',
  components: { Protocol, Account, AccessAuth },
  data() {
    return {
      visible: false,
      type: 'create',
      assetId: null,
      loading: false,
      baseForm: {
        name: '',
        ip: '',
        parent_id: undefined,
        comment: '',
      },
      baseRules: {
        name: [{ required: true, message: `${this.$t(`placeholder1`)}` }],
        parent_id: [{ required: true, message: `${this.$t(`placeholder2`)}` }],
      },
      nodeList: [],
    }
  },
  computed: {
    title() {
      if (this.type === 'create') {
        return this.$t(`oneterm.assetList.createAsset`)
      }
      return this.$t(`oneterm.assetList.editAsset`)
    },
  },
  methods: {
    setAsset(asset, type) {
      this.visible = true
      this.type = type
      getNodeList().then((res) => {
        const tree = this.formatTree(res?.data?.list || [])
        this.nodeList = tree
      })
      this.$nextTick(() => {
        const {
          id = null,
          name = '',
          ip = '',
          comment = '',
          parent_id,
          gateway_id = undefined,
          protocols = [],
          authorization = {},
          access_auth = {},
        } = asset ?? {}
        this.assetId = id
        this.baseForm = {
          name,
          ip,
          comment,
          parent_id: parent_id || undefined,
        }
        this.$refs.protocol.setValues({ gateway_id, protocols })
        this.$refs.account.setValues({ authorization })
        this.$refs.accessAuth.setValues(access_auth)
      })
    },

    formatTree(data) {
      const tree = []
      const lookup = {}

      data.forEach(item => {
        lookup[item.id] = { ...item, children: [] }
      })

      data.forEach(item => {
        if (item.parent_id === 0) {
          tree.push(lookup[item.id])
        } else if (lookup[item.parent_id]) {
          lookup[item.parent_id].children.push(lookup[item.id])
        }
      })

      return tree
    },

    handleSubmit() {
      this.$refs.baseForm.validate((valid) => {
        if (valid) {
          const { name, ip, parent_id, comment } = this.baseForm
          const { gateway_id, protocols } = this.$refs.protocol.getValues()
          const { authorization } = this.$refs.account.getValues()
          const access_auth = this.$refs.accessAuth.getValues()
          const params = {
            name,
            ip: ip?.trim?.() ?? '',
            comment,
            parent_id: parent_id ?? 0,
            protocols,
            gateway_id,
            authorization,
            access_auth,
          }
          this.loading = true
          if (this.assetId) {
            putAssetById(this.assetId, { ...params, id: this.assetId })
              .then((res) => {
                this.$message.success(this.$t('editSuccess'))
                this.$emit('submitAsset', true, parent_id)
                this.visible = false
              })
              .finally(() => {
                this.loading = false
              })
          } else {
            postAsset(params)
              .then((res) => {
                this.$message.success(this.$t('createSuccess'))
                this.$emit('submitAsset', true)
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
.asset-create-asset {
  width: 100%;
  height: calc(100vh - 112px);
  margin-bottom: -24px;
  background-color: #ffffff;
  border-radius: 15px;
  position: relative;
  .asset-create-asset-container {
    height: calc(100% - 56px);
    padding: 18px 5vw;
    overflow: auto;
    strong {
      color: #000;
    }
  }
  .asset-create-asset-footer {
    width: 100%;
    padding: 12px 5vw;
    position: absolute;
    bottom: 0;
    left: 0;
    box-shadow: 0px -1px 0px #e4eaf7;
    text-align: right;
  }
}
</style>
