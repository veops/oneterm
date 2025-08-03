<template>
  <a-modal
    width="900px"
    :visible="visible"
    :title="title"
    @cancel="handleCancel"
    @ok="handleOk"
    :confirmLoading="loading"
  >
    <AccessAuth v-if="type === 'access_auth'" ref="accessAuth" />
    <Account v-else-if="type === 'account_ids'" ref="account" :colSpan="{ span: 24 }" />
    <Protocol v-else ref="protocol" />
  </a-modal>
</template>

<script>
import Protocol from './protocol/index.vue'
import Account from './account.vue'
import AccessAuth from './accessAuth.vue'
import { putAssetById } from '@/modules/oneterm/api/asset'
export default {
  name: 'BatchUpdateModal',
  components: { Protocol, Account, AccessAuth },
  props: {
    selectedRowKeys: {
      type: Array,
      default: () => [],
    },
  },
  data() {
    return {
      visible: false,
      type: 'access_auth',
      loading: false,
    }
  },
  computed: {
    title() {
      switch (this.type) {
        case 'access_auth':
          return this.$t('grant')
        case 'account_ids':
          return this.$t('oneterm.assetList.addAccount')
        default:
          return this.$t('oneterm.assetList.editProtocol')
      }
    },
  },
  methods: {
    open(type) {
      this.visible = true
      this.type = type
      this.$nextTick(() => {
        this.resetRef()
      })
    },
    handleCancel() {
      this.resetRef()
      this.visible = false
    },
    resetRef() {
      if (this.type === 'access_auth') {
        this.$refs.accessAuth.setValues({})
      } else if (this.type === 'account_ids') {
        this.$refs.account.setValues({ authorization: {} })
      } else if (this.type === 'protocols') {
        this.$refs.protocol.setValues({ gateway_id: undefined, protocols: [] })
      }
    },
    async handleOk() {
      this.loading = true
      for (let i = 0; i < this.selectedRowKeys.length; i++) {
        const params = { ...this.selectedRowKeys[i] }
        if (this.type === 'access_auth') {
          const { cmd_ids, template_ids, time_ranges, timezone } = this.$refs.accessAuth.getValues()
          params.access_time_control = {
            time_ranges,
            timezone
          }
          params.asset_command_control = {
            cmd_ids,
            template_ids
          }
        } else if (this.type === 'account_ids') {
          const { authorization } = this.$refs.account.getValues()
          params.authorization = authorization
        } else if (this.type === 'protocols') {
          const { gateway_id, protocols } = this.$refs.protocol.getValues()
          params.gateway_id = gateway_id
          params.protocols = protocols
        }
        await putAssetById(this.selectedRowKeys[i].id, params)
      }
      this.loading = false
      this.$message.success(this.$t('editSuccess'))
      this.$emit('submit')
      this.handleCancel()
    },
  },
}
</script>

<style></style>
