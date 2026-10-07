<template>
  <a-modal
    :visible="visible"
    title=""
    :width="600"
    :footer="null"
    @cancel="handleCancel"
  >
    <div class="mfa">
      <InstallApp
        v-if="step === 'installApp'"
        @next="step = 'bindMFA'"
      />

      <BindMFA
        v-if="step === 'bindMFA'"
        :otp_uri="otp_uri"
        :otp_secret_key="otp_secret_key"
        @submit="submit"
      />
    </div>
  </a-modal>
</template>

<script>
import moment from 'moment'
import { mapState, mapMutations } from 'vuex'
import { MFAChallenge, MFAVerify } from '@/api/mfa.js'

import InstallApp from './installApp.vue'
import BindMFA from './bindMFA'

const TTL_SECOND = 300

export default {
  name: 'MFAModal',
  components: {
    InstallApp,
    BindMFA
  },
  props: {
    enableTTLCheck: {
      type: Boolean,
      default: true
    }
  },
  data() {
    return {
      visible: false,
      step: 'installApp',

      otp_uri: '',
      otp_secret_key: '',
      otp_token_hash: '',

      action: '',
      scope: '',
      generation: 0
    }
  },
  computed: {
    ...mapState({
      mfa_token: (state) => state.user.mfa_token,
      mfaTokens: (state) => state.user.mfa_tokens || {}
    })
  },
  beforeDestroy() { this.generation += 1 },
  methods: {
    ...mapMutations(['SET_MFA_TOKEN']),
    async open({
      action,
      scope,
      force = false
    }) {
      const generation = ++this.generation
      const cached = this.mfaTokens[scope] || this.mfa_token
      // If the cache token has not expired, there is no need to re-verify it; simply return it.
      if (
        !force && this.enableTTLCheck &&
        cached?.valid_time > moment().unix() &&
        cached?.value &&
        cached?.scope === scope
      ) {
        this.$emit('ok', {
          token: cached.value,
          code: '',
          action
        })
        return
      }

      this.action = action
      this.scope = scope
      const res = await MFAChallenge({
        scope,
        ttl_seconds: TTL_SECOND
      })

      if (generation !== this.generation) return

      if (res.mfa_required === false) {
        this.$emit('ok', { token: '', code: '', action })
        return
      }

      this.visible = true
      this.step = Number(res?.bind_otp) === 1 ? 'installApp' : 'bindMFA'
      this.otp_uri = res.otp_uri
      this.otp_secret_key = res.otp_secret_key
      this.otp_token_hash = res.otp_token_hash

      this.action = action
      this.scope = scope
    },
    handleCancel() {
      const generation = ++this.generation
      this.visible = false

      // Prevent switching before the pop-up window closing animation has finished. Delay for 300 milliseconds.
      setTimeout(() => {
        if (generation === this.generation) this.step = 'installApp'
      }, 300)
      this.$emit('cancel')
    },
    closeModal() {
      const generation = ++this.generation
      this.visible = false
      setTimeout(() => {
        if (generation === this.generation) this.step = 'installApp'
      }, 300)
    },
    async submit(otp_code) {
      const { generation, action, scope } = this
      const res = await MFAVerify({
        otp_token_hash: this.otp_token_hash,
        otp_code
      })
      if (generation !== this.generation) return
      if (res.mfa_token) {
        if (this.enableTTLCheck) {
          const valid_time = moment().unix() + Number(res.expires_in || TTL_SECOND)
          this.SET_MFA_TOKEN({
            value: res.mfa_token,
            valid_time,
            scope
          })
        }

        this.closeModal()
        this.$emit('ok', {
          token: res.mfa_token,
          code: otp_code,
          action
        })
      }
    }
  }
}
</script>

<style lang="less" scoped>
.mfa {
  width: 100%;
  height: 100%;
}
</style>
