<template>
  <a-modal :title="title" :visible="visible" @cancel="handleCancel" @ok="handleOk" :confirmLoading="loading">
    <a-form-model ref="gatewayForm" :model="form" :rules="rules" :label-col="{ span: 6 }" :wrapper-col="{ span: 16 }">
      <a-form-model-item :label="`${$t('oneterm.assetList.gatewayName')}`" prop="name">
        <a-input v-model="form.name" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
      <a-form-model-item :label="$t(`oneterm.host`)" prop="host">
        <a-input v-model="form.host" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
      <a-form-model-item :label="$t(`oneterm.port`)" prop="port">
        <a-input v-model="form.port" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
      <a-form-model-item prop="account" :label-col="{ span: 7 }" :wrapper-col="{ span: 14 }">
        <template slot="label">
          <a-tooltip v-if="form.account_type === 2" :title="$t('oneterm.assetList.gatewayAccountTip')">
            <a><a-icon type="question-circle"/></a>
          </a-tooltip>
          {{ $t(`oneterm.account`) }}</template
          >
        <a-input v-model="form.account" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
      <CredentialEditor
        v-model="form"
        :editing="!!form.id"
        :replace.sync="replaceCredential"
        :original-type="originalType"
        :require-password="false"
      />
    </a-form-model>
  </a-modal>
</template>

<script>
import { postGateway, putGatewayById } from '../../../api/gateway'
import CredentialEditor from '../../../components/credentialEditor.vue'

export default {
  name: 'GatewayModal',
  components: { CredentialEditor },
  data() {
    return {
      visible: false,
      replaceCredential: false,
      originalType: 1,
      form: {
        name: '',
        host: '',
        port: '',
        account_type: 1,
        account: '',
        password: '',
        pk: '',
        phrase: '',
      },
      rules: {
        name: [{ required: true, message: `${this.$t(`placeholder1`)}` }],
        host: [{ required: true, message: `${this.$t(`placeholder1`)}` }],
        port: [
          {
            required: true,
            message: `${this.$t(`placeholder1`)}`,
            pattern: RegExp('^[0-9]+$'),
          },
        ],
      },
      loading: false,
    }
  },
  computed: {
    title() {
      if (this.form.id) {
        return this.$t('oneterm.assetList.editGateway')
      }
      return this.$t('oneterm.assetList.createGateway')
    },
  },
  methods: {
    open(data) {
      this.replaceCredential = false
      this.originalType = Number(data?.account_type) || 1
      this.form = {
        id: data?.id,
        name: data?.name || '',
        host: data?.host || '',
        port: data?.port || '',
        account: data?.account || '',
        account_type: this.originalType,
        password: '',
pk: '',
phrase: ''
      }
      this.visible = true
      this.$nextTick(() => this.$refs.gatewayForm.clearValidate())
    },
    handleCancel() {
      this.$refs.gatewayForm.resetFields()
      this.form = {
        name: '',
        host: '',
        port: '',
        account_type: 1,
        account: '',
        password: '',
        pk: '',
        phrase: '',
      }
      this.visible = false
    },
    async handleOk() {
      this.$refs.gatewayForm.validate(async (valid) => {
        if (valid) {
          this.loading = true
          const { name, host, port, account_type, account, password, pk, phrase } = this.form
          const params = { name, host, account_type, account, port: Number(port) }
          if (!this.form.id || this.replaceCredential) {
            if (account_type === 1) {
              params.password = password
            } else {
              params.pk = pk
              params.phrase = phrase
            }
          }
          if (this.form.id) {
            await putGatewayById(this.form.id, params)
              .then(() => {
                this.$message.success(this.$t('editSuccess'))
              })
              .finally(() => {
                this.loading = false
              })
          } else {
            await postGateway(params)
              .then(() => {
                this.$message.success(this.$t('createSuccess'))
              })
              .finally(() => {
                this.loading = false
              })
          }
          this.$emit('submit')
          this.handleCancel()
        }
      })
    },
  },
}
</script>

<style></style>
