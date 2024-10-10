<template>
  <a-modal :visible="visible" :title="$t(`login`)" @cancel="handleCancel" @ok="handleOk">
    <a-form-model
      ref="loginForm"
      :model="loginForm"
      :rules="rules"
      :label-col="{ span: 6 }"
      :wrapper-col="{ span: 18 }"
      class="asset-login-modal"
    >
      <a-form-model-item class="asset-login-account" :label="$t('oneterm.account')" prop="account_id">
        <a-checkbox-group v-if="choiceAccountByCheckbox" v-model="loginForm.account_ids" >
          <a-checkbox v-for="item in filterAccountList" :key="item.id" :value="item.id">
            <a-tooltip :title="item.name" >
              <span class="asset-login-account-choice">{{ item.name }}</span>
            </a-tooltip>
          </a-checkbox>
        </a-checkbox-group>
        <a-radio-group v-else v-model="loginForm.account_id">
          <a-radio v-for="item in filterAccountList" :key="item.id" :value="item.id">
            <a-tooltip :title="item.name" >
              <span class="asset-login-account-choice">{{ item.name }}</span>
            </a-tooltip>
          </a-radio>
        </a-radio-group>
      </a-form-model-item>
      <a-form-model-item v-if="showProtocol" :label="$t(`oneterm.port`)" prop="protocol">
        <a-radio-group v-model="loginForm.protocol">
          <a-radio v-for="item in protocols" :key="item" :value="item">
            {{ item }}
          </a-radio>
        </a-radio-group>
      </a-form-model-item>
    </a-form-model>
  </a-modal>
</template>

<script>
import { mapState } from 'vuex'
import { getAccountList } from '../../../api/account'

export default {
  name: 'LoginModal',
  props: {
    showProtocol: {
      type: Boolean,
      default: true,
    },
    choiceAccountByCheckbox: {
      type: Boolean,
      default: false,
    },
    forMyAsset: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      visible: false,
      loginForm: {
        account_id: '',
        protocol: '',
        account_ids: []
      },
      rules: {
        account_id: [{ required: true, message: `${this.$t(`placeholder2`)}`, trigger: 'change' }],
        protocol: [{ required: true, message: `${this.$t(`placeholder2`)}`, trigger: 'change' }],
        account_ids: [{ required: true, message: `${this.$t(`placeholder2`)}`, trigger: 'change' }],
      },
      asset_id: null,
      asset_name: '',
      authorization: {},
      accountList: [],
      protocols: []
    }
  },
  computed: {
    ...mapState({
      rid: (state) => state.user.rid,
      roles: (state) => state.user.roles,
    }),
    filterAccountList() {
      const _filterAccountList = []
      Object.keys(this.authorization).forEach((acc_id) => {
        const _find = this.accountList.find((item) => Number(item.id) === Number(acc_id))
        if (_find) {
          _filterAccountList.push(_find)
        }
      })
      return _filterAccountList
    },
  },
  mounted() {
    getAccountList({ page_index: 1, info: this.forMyAsset }).then((res) => {
      this.accountList = res?.data?.list || []
    })
  },
  methods: {
    open(asset_id, asset_name, authorization, protocols) {
      this.visible = true
      this.asset_id = asset_id
      this.asset_name = asset_name
      this.authorization = authorization
      this.protocols = protocols
      this.loginForm = {
        account_id: this.filterAccountList[0]?.id,
        protocol: protocols[0],
        account_ids: [this.filterAccountList[0]?.id]
      }
    },
    handleCancel() {
      this.visible = false
    },
    handleOk() {
      this.$refs.loginForm.validate((valid) => {
        if (valid) {
          this.handleCancel()
          const { account_id, protocol, account_ids } = this.loginForm
          const protocolType = protocol.split?.(':')?.[0] || ''

          if (this.choiceAccountByCheckbox) {
            if (account_ids.length) {
              this.$emit('openTerminalList', {
                accountList: account_ids.map((id) => id),
                protocol,
                assetId: this.asset_id,
                assetName: this.asset_name,
                protocolType
              })
            }
          } else {
            this.$emit('openTerminal', {
              assetId: this.asset_id,
              assetName: this.asset_name,
              accountId: account_id,
              protocol,
              protocolType,
            })
          }
        }
      })
    },
  },
}
</script>

<style></style>
