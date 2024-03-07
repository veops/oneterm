<template>
  <a-modal :visible="visible" :title="$t(`login`)" @cancel="handleCancel" @ok="handleOk">
    <a-form-model
      ref="loginForm"
      :model="loginForm"
      :rules="rules"
      :label-col="{ span: 6 }"
      :wrapper-col="{ span: 18 }"
    >
      <a-form-model-item :label="$t('oneterm.account')" prop="account_id">
        <a-radio-group v-model="loginForm.account_id">
          <a-radio v-for="item in filterAccountList" :key="item.id" :value="item.id">
            {{ item.name }}
          </a-radio>
        </a-radio-group>
      </a-form-model-item>
      <a-form-model-item :label="$t(`oneterm.port`)" prop="protocol">
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
import { postConnectIsRight } from '../../../api/connect'
export default {
  name: 'LoginModal',
  data() {
    return {
      visible: false,
      loginForm: {
        account_id: '',
        protocol: '',
      },
      rules: {
        account_id: [{ required: true, message: `${this.$t(`placeholder2`)}`, trigger: 'change' }],
        protocol: [{ required: true, message: `${this.$t(`placeholder2`)}`, trigger: 'change' }],
      },
      asset_id: null,
      authorization: {},
      accountList: [],
      protocols: [],
    }
  },
  computed: {
    ...mapState({
      rid: (state) => state.user.rid,
      roles: (state) => state.user.roles,
    }),
    filterAccountList() {
      const _filterAccountList = []
      Object.entries(this.authorization).forEach(([acc_id, rids]) => {
        if (
          rids.includes(this.rid) ||
          this.roles.permissions.includes('acl_admin') ||
          this.roles.permissions.includes('oneterm_admin')
        ) {
          const _find = this.accountList.find((item) => item.id === Number(acc_id))
          if (_find) {
            _filterAccountList.push(_find)
          }
        }
      })
      return _filterAccountList
    },
  },
  mounted() {
    getAccountList({ page_index: 1, info: true }).then((res) => {
      this.accountList = res?.data?.list || []
    })
  },
  methods: {
    open(asset_id, authorization, protocols) {
      this.visible = true
      this.asset_id = asset_id
      this.authorization = authorization
      this.protocols = protocols
      this.loginForm = {
        account_id: this.filterAccountList[0]?.id,
        protocol: protocols[0],
      }
    },
    handleCancel() {
      this.visible = false
    },
    handleOk() {
      this.$refs.loginForm.validate((valid) => {
        if (valid) {
          this.handleCancel()
          const { account_id, protocol } = this.loginForm
          if (protocol.includes('rdp') || protocol.includes('vnc')) {
            window.open(`/oneterm/guacamole/${this.asset_id}/${account_id}/${protocol}`, '_blank')
          } else {
            postConnectIsRight(this.asset_id, account_id, protocol).then((res) => {
              if (res?.data?.session_id) {
                window.open(`/oneterm/terminal?session_id=${res?.data?.session_id}`, '_blank')
              }
            })
          }
        }
      })
    },
  },
}
</script>

<style></style>
