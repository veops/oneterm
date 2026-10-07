<template>
  <a-modal
    :visible="visible"
    :title="$t(attachTo ? 'oneterm.pam.attachBinding' : 'oneterm.pam.adoptAccount')"
    :confirm-loading="saving"
    :width="600"
    @ok="submit"
    @cancel="close">
    <a-form layout="vertical">
      <a-form-item :label="$t('oneterm.pam.sourceAccount')" required>
        <a-input :value="source ? source.name + ' (' + source.account + ')' : ''" disabled />
      </a-form-item>
      <a-form-item :label="$t('oneterm.asset')" required>
        <a-select
          v-model="form.asset_id"
          show-search
          :filter-option="false"
          :loading="assetLoading"
          @search="searchAssets"
          @change="assetChanged">
          <a-select-option v-for="option in assets" :key="option.id">{{ option.name }} ({{ option.ip }})</a-select-option>
        </a-select>
      </a-form-item>
      <template v-if="!attachTo">
        <a-form-item :label="$t('oneterm.pam.accountAuthority')" required>
          <a-radio-group v-model="form.authority_kind"><a-radio value="asset_local">{{ $t('oneterm.pam.localAccount') }}</a-radio><a-radio value="shared" :disabled="Boolean(platform && platform.local_authority_only)">{{ $t('oneterm.pam.sharedAccount') }}</a-radio></a-radio-group>
        </a-form-item>
        <a-form-item v-if="form.authority_kind === 'shared'" :label="$t('oneterm.pam.authorityReference')" required><a-input v-model="form.authority_ref" :max-length="128" /></a-form-item>
        <a-form-item :label="$t('oneterm.pam.accountPlatform')" required>
          <a-select v-model="form.platform" @change="platformChanged"><a-select-option v-for="item in platforms" :key="item.id">{{ $t('oneterm.pam.platforms.' + item.id) }}</a-select-option></a-select>
        </a-form-item>
        <a-form-item v-if="platform && platform.requires_qualifier" :label="$t('oneterm.pam.nativeQualifier')" required><a-input v-model="form.native_qualifier" :max-length="255" /></a-form-item>
      </template>
    </a-form>
  </a-modal>
</template>

<script>
import { getPAMAdoptionAssets } from '@/modules/oneterm/api/pam'
import { adoptionPayload } from './managedAccountForm'

export default {
  name: 'PAMManagedAccountEditor',
  data() {
    return { visible: false,
      saving: false,
      form: {},
      attachTo: null,
      assets: [],
      source: null,
      asset: null,
      platforms: [],
      assetLoading: false,
      assetGeneration: 0,
      assetTimer: null }
  },
  computed: { platform() { return this.platforms.find((item) => item.id === this.form.platform) } },
  beforeDestroy() { this.close() },
  methods: {
    open(source, capabilities, attachTo = null) {
      this.close()
      this.attachTo = attachTo
      this.platforms = capabilities.managed_account_platforms || []
      this.form = { account_id: source?.id,
        asset_id: undefined,
        authority_kind: 'asset_local',
        authority_ref: '',
        platform: this.platforms[0]?.id,
        native_qualifier: '' }
      this.source = source; this.asset = null; this.assets = []
      this.visible = true
      this.loadAssets('')
    },
    close() {
      this.visible = false; this.saving = false
      this.assetGeneration += 1
      clearTimeout(this.assetTimer)
      this.$emit('cancel')
    },
    finish(success) { this.saving = false; if (success) this.close() },
    assetChanged(id) { this.asset = this.assets.find((item) => item.id === id) },
    platformChanged() {
      if (this.platform?.local_authority_only) this.form.authority_kind = 'asset_local'
      if (!this.platform?.requires_qualifier) this.form.native_qualifier = ''
    },
    searchAssets(search) { clearTimeout(this.assetTimer); this.assetTimer = setTimeout(() => this.loadAssets(search), 200) },
    async loadAssets(search) {
      const generation = ++this.assetGeneration
      this.assetLoading = true
      try {
        const response = await getPAMAdoptionAssets({ search, page_index: 1, page_size: 50 })
        if (generation !== this.assetGeneration || !this.visible) return
        const options = response.data?.list || []
        this.assets = this.asset && !options.some((item) => item.id === this.asset.id) ? [this.asset, ...options] : options
      } catch (error) {
        // Keep the selected asset if a search fails.
      } finally { if (generation === this.assetGeneration) this.assetLoading = false }
    },
    submit() {
      if (this.saving) return
      if (!this.form.asset_id || !this.form.account_id) { this.$message.error(this.$t('oneterm.pam.adoptionInvalid')); return }
      if (this.attachTo) {
        this.saving = true
        this.$emit('submit', { type: 'attach', id: this.attachTo.id, data: { asset_id: this.form.asset_id, account_id: this.form.account_id } })
        return
      }
      let data
      try { data = adoptionPayload(this.form) } catch (error) { this.$message.error(this.$t(error.message)); return }
      if (this.platform?.requires_qualifier && !data.native_qualifier) { this.$message.error(this.$t('oneterm.pam.qualifierRequired')); return }
      if (this.platform?.password_only && Number(this.source?.account_type) !== 1) { this.$message.error(this.$t('oneterm.pam.passwordAccountRequired')); return }
      this.saving = true
      this.$emit('submit', { type: 'adopt', data })
    }
  }
}
</script>
