<template>
  <span class="credential-reveal">
    <a-tooltip :title="buttonLabel || viewLabel">
      <a-button
        :type="buttonLabel ? 'default' : 'link'"
        size="small"
        :aria-label="buttonLabel || viewLabel"
        :loading="loading"
        :disabled="disabled"
        @click="open">
        <a-icon v-if="!loading" type="eye-invisible" />
        {{ buttonLabel }}
      </a-button>
    </a-tooltip>
    <span v-if="!buttonLabel">******</span>
    <a-modal
      :visible="visible"
      :title="name ? name + ' - ' + viewLabel : viewLabel"
      :width="640"
      :footer="null"
      :destroy-on-close="true"
      @cancel="clear">
      <div v-for="field in fields" :key="field.key" class="credential-value">
        <div class="credential-value-header">
          <label>{{ field.label }}</label>
          <a-tooltip :title="$t('copy')">
            <a-button type="link" size="small" @click="copy(field.value)"><a-icon type="copy" /></a-button>
          </a-tooltip>
        </div>
        <a-textarea
          :value="field.value"
          :auto-size="{ minRows: 1, maxRows: 10 }"
          readonly
          autocomplete="off"
          spellcheck="false"
          class="credential-value-input"
        />
      </div>
    </a-modal>
    <MFAModal ref="mfa" @ok="verified" @cancel="clear" />
    <PasswordViewAccess
      ref="access"
      :id="id"
      :kind="kind"
      :name="name"
      :title="viewLabel"
      @change="$emit('change', $event)"
      @ready="requestMFA"
      @cancel="clear" />
  </span>
</template>

<script>
import MFAModal from '@/views/mfa/mfaModal'
import PasswordViewAccess from './passwordViewAccess.vue'

export default {
  name: 'CredentialReveal',
  components: { MFAModal, PasswordViewAccess },
  props: {
    id: { type: Number, required: true },
    kind: { type: String, default: 'account' },
    name: { type: String, default: '' },
    authType: { type: Number, default: 0 },
    buttonLabel: { type: String, default: '' },
    disabled: { type: Boolean, default: false },
    load: { type: Function, required: true },
    scope: { type: String, default: 'oneterm_pam_retrieve' }
  },
  data() {
    return { visible: false, loading: false, material: null, generation: 0, retried: false, timer: null }
  },
  computed: {
    viewLabel() {
      return this.$t(this.authType === 1 ? 'oneterm.credential.viewPassword' : this.authType === 2 ? 'oneterm.credential.viewPrivateKey' : 'oneterm.credential.viewLoginInfo')
    },
    fields() {
      if (!this.material) return []
      if (this.material.account_type === 1) {
        return [{ key: 'password', label: this.$t('oneterm.password'), value: this.material.password || '' }]
      }
      return [
        { key: 'pk', label: this.$t('oneterm.credential.privateKey'), value: this.material.pk || '' },
        { key: 'phrase', label: this.$t('oneterm.phrase'), value: this.material.phrase || '' }
      ]
    }
  },
  watch: {
    id: 'clear',
    kind: 'clear',
    authType: 'clear',
    disabled(value) { if (value) this.clear() }
  },
  mounted() {
    document.addEventListener('visibilitychange', this.onVisibilityChange)
  },
  deactivated() { this.clear() },
  beforeDestroy() {
    this.clear()
    document.removeEventListener('visibilitychange', this.onVisibilityChange)
  },
  methods: {
    clear() {
      this.generation += 1
      this.visible = false
      this.loading = false
      this.material = null
      clearTimeout(this.timer)
      this.timer = null
      if (this.$refs.mfa) this.$refs.mfa.closeModal()
      if (this.$refs.access) this.$refs.access.close()
    },
    onVisibilityChange() {
      if (document.hidden) this.clear()
    },
    async open() {
      if (this.disabled || this.loading) return
      this.clear()
      this.retried = false
      this.loading = true
      const generation = this.generation
      try {
        const ready = await this.$refs.access.open(generation)
        if (generation !== this.generation || this.disabled) return
        if (ready) await this.requestMFA({ action: generation })
        else this.loading = false
      } catch (error) {
        if (generation === this.generation) this.clear()
      }
    },
    async requestMFA({ action }) {
      if (action !== this.generation || this.disabled) return
      this.loading = true
      try { await this.$refs.mfa.open({ scope: this.scope, action }) } catch (error) { if (action === this.generation) this.clear() }
    },
    async verified({ token, action }) {
      const generation = this.generation
      if (action !== generation || this.disabled) return
      try {
        const result = await this.load(this.id, token)
        if (generation !== this.generation || this.disabled) return
        this.material = {
          account_type: Number(result.account_type), password: result.password, pk: result.pk, phrase: result.phrase
        }
        this.visible = true
        this.loading = false
        this.timer = setTimeout(this.clear, 60000)
      } catch (error) {
        if (generation !== this.generation) return
        if (Number(error?.response?.data?.code) === 4414) {
          this.loading = false
          this.$refs.mfa.closeModal()
          await this.$refs.access.open(generation)
          return
        }
        if (Number(error?.response?.data?.code) === 4403 && !this.retried) {
          this.retried = true
          try {
            await this.$refs.mfa.open({ scope: this.scope, action: generation, force: true })
          } catch (challengeError) {
            if (generation === this.generation) this.clear()
          }
          return
        }
        this.clear()
        this.$message.error(error?.response?.data?.message || this.$t('requestError'))
      }
    },
    async copy(value) {
      try {
        await this.$copyText(value)
        this.$message.success(this.$t('copySuccess'))
      } catch (error) {
        this.$message.error(this.$t('requestError'))
      }
    }
  }
}
</script>

<style scoped>
.credential-reveal { display: inline-flex; align-items: center; gap: 6px; }
.credential-value + .credential-value { margin-top: 16px; }
.credential-value-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px; }
.credential-value-input { font-family: monospace; font-size: 13px; }
</style>
