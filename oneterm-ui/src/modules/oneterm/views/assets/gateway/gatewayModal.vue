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
      <a-form-model-item :label="$t(`oneterm.accountType`)" prop="account_type">
        <a-radio-group v-model="form.account_type">
          <a-radio :value="1">
            {{ $t('oneterm.password') }}
          </a-radio>
          <a-radio :value="2">
            {{ $t('oneterm.secretkey') }}
          </a-radio>
        </a-radio-group>
      </a-form-model-item>
      <a-form-model-item prop="account" :label-col="{ span: 7 }" :wrapper-col="{ span: 14 }">
        <template slot="label">
          <a-tooltip v-if="form.account_type === 2" :title="$t('oneterm.assetList.gatewayAccountTip')">
            <a><a-icon type="question-circle"/></a>
          </a-tooltip>
          {{ $t(`oneterm.account`) }}
        </template>
        <a-input v-model="form.account" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
      <a-form-model-item prop="password" :label-col="{ span: 7 }" :wrapper-col="{ span: 14 }">
        <template slot="label">
          <a-tooltip
            :overlayStyle="{ whiteSpace: 'break-spaces' }"
            v-if="form.account_type === 2"
            :title="$t('oneterm.assetList.gatewaySecretkeyTip')"
          >
            <a><a-icon type="question-circle"/></a>
          </a-tooltip>
          {{ form.account_type === 1 ? $t('oneterm.password') : $t('oneterm.secretkey') }}
        </template>
        <a-input-password v-if="form.account_type === 1" v-model="form.password" :placeholder="`${$t(`placeholder1`)}`" />
        <a-textarea v-else v-model="form.pk" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
      <a-form-model-item
        prop="phrase"
        :label-col="{ span: 7 }"
        :wrapper-col="{ span: 14 }"
        v-if="form.account_type === 2"
      >
        <template slot="label">
          <a-tooltip :title="$t('oneterm.assetList.gatewayPhraseTip')">
            <a><a-icon type="question-circle"/></a>
          </a-tooltip>
          {{ $t('oneterm.phrase') }}
        </template>
        <a-input-password v-model="form.phrase" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
    </a-form-model>
  </a-modal>
</template>

<script>
import { postGateway, putGatewayById } from '../../../api/gateway'

export default {
  name: 'GatewayModal',
  data() {
    return {
      visible: false,
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
      this.visible = true
      if (data) {
        this.form = { ...data }
      }
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
          if (account_type === 1) {
            params.password = password
          } else {
            params.pk = pk
            params.phrase = phrase
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
