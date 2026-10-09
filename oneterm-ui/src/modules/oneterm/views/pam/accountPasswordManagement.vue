<template>
  <div>
    <a-drawer :visible="detailVisible" width="min(820px, 100vw)" :title="account ? account.name : $t('oneterm.pam.managedAccountScope')" :body-style="{ height: 'calc(100% - 55px)', overflow: 'hidden' }" @close="closeDetail">
      <a-spin :spinning="detailLoading" class="pam-managed-detail-spin">
        <template v-if="account && capabilities">
          <div class="pam-managed-summary"><strong>{{ account.account }}</strong><span>{{ authority(account) }}</span><span>{{ $t('oneterm.pam.platforms.' + account.platform) }}</span></div>
          <div class="pam-managed-detail-actions">
            <span class="pam-credential-view">
              <a v-if="allowed(account, 'read')" @click="showCredential"><a-icon :type="credentialVisible ? 'eye' : 'eye-invisible'" /></a>
              <span v-if="credentialVisible">{{ credentialText }}</span>
              <span v-else>******</span>
              <a v-if="credentialVisible" @click="copyCredential"><a-icon type="copy" /></a>
            </span>
            <a-space>
              <a-tooltip :title="$t('grant')"><a-button :disabled="!allowed(account, 'grant')" @click="grant(account)"><a-icon type="team" /></a-button></a-tooltip>
              <a-dropdown v-if="allowed(account, 'manage_policy')" :trigger="['click']" :disabled="Boolean(pending)">
                <a-button :title="$t('operation')"><a-icon type="ellipsis" /></a-button>
                <a-menu slot="overlay">
                  <a-menu-item @click="toggleAccount(account)">{{ $t(account.enabled ? 'oneterm.pam.disableManagedAccount' : 'oneterm.pam.enableManagedAccount') }}</a-menu-item>
                  <a-menu-item v-if="allowed(account, 'grant')" @click="removeManagement">{{ $t('oneterm.pam.removeManagement') }}</a-menu-item>
                </a-menu>
              </a-dropdown>
            </a-space>
          </div>
          <a-tabs v-model="detailTab" :animated="false" class="pam-managed-tabs">
            <a-tab-pane key="bindings" :tab="$t('oneterm.pam.connectionBindings')">
              <div class="pam-managed-toolbar"><span>{{ $t('oneterm.pam.bindingCount', { count: bindingTotal }) }}</span><a-button :disabled="!allowed(account, 'manage_policy') || !allowed(account, 'grant') || Boolean(pending)" @click="attach"><a-icon type="plus" />{{ $t('oneterm.pam.attachBinding') }}</a-button></div>
              <ops-table
                :data="bindings"
                :height="Math.min(420, tableHeight)"
                :row-config="{ keyField: 'id' }"
                :scroll-y="{ enabled: true, gt: 20 }"
                stripe
                resizable
                show-overflow
                size="small">
                <vxe-column :title="$t('oneterm.asset')" min-width="130"><template #default="{ row }">{{ row.asset_name || '#' + row.asset_id }}</template></vxe-column>
                <vxe-column :title="$t('oneterm.pam.sourceAccount')" min-width="120"><template #default="{ row }">{{ row.account_name || '#' + row.account_id }}</template></vxe-column>
                <vxe-column field="target_host" :title="$t('oneterm.pam.boundTarget')" min-width="130" />
                <vxe-column :title="$t('oneterm.protocol')" min-width="110"><template #default="{ row }">{{ (row.protocols || []).join(', ') }}</template></vxe-column>
                <vxe-column :title="$t('status')" width="110"><template #default="{ row }"><span :class="row.route_changed ? 'pam-managed-warning' : ''">{{ $t(row.route_changed ? 'oneterm.pam.bindingNeedsReview' : row.enabled ? 'oneterm.enabled' : 'oneterm.disabled') }}</span></template></vxe-column>
                <vxe-column :title="$t('operation')" width="86" fixed="right"><template #default="{ row }"><a-space>
                  <a-tooltip v-if="row.route_changed" :title="$t('oneterm.pam.reviewBinding')"><a-button type="link" size="small" :disabled="!allowed(account, 'manage_policy') || Boolean(pending)" @click="review(row, row.enabled)"><a-icon type="sync" /></a-button></a-tooltip>
                  <a-tooltip :title="$t(row.enabled ? 'oneterm.disabled' : 'oneterm.enabled')"><a-button type="link" size="small" :disabled="!allowed(account, 'manage_policy') || Boolean(pending)" @click="review(row, !row.enabled)"><a-icon type="poweroff" /></a-button></a-tooltip>
                </a-space></template></vxe-column>
              </ops-table>
              <div class="pam-managed-pagination"><a-pagination size="small" :current="bindingPage" :page-size="50" :total="bindingTotal" @change="loadBindings" /></div>
            </a-tab-pane>
            <a-tab-pane key="policy" :tab="$t('oneterm.pam.accessPolicy')">
              <PolicyEditor
                v-if="policy"
                :policy="policy"
                :capabilities="capabilities"
                :disabled="!allowed(account, 'manage_policy') || !allowed(account, 'grant')"
                :saving="Boolean(pending)"
                @save="savePolicy" />
            </a-tab-pane>
            <a-tab-pane v-if="passwordPlatform" key="password" :tab="$t('oneterm.pam.password.title')">
              <PasswordOperations v-if="detailTab === 'password'" :account="account" :capabilities="capabilities" :is-admin="isAdmin" @updated="inspect(account.id, true)" />
            </a-tab-pane>
          </a-tabs>
        </template>
      </a-spin>
    </a-drawer>
    <ManagedAccountEditor ref="editor" @submit="begin" @cancel="cancelOperation" />
    <MFAModal ref="mfa" @ok="verified" @cancel="cancelOperation" />
    <GrantModal ref="grant" />
  </div>
</template>

<script>
import { mapState } from 'vuex'
import { getAccountByCredentials } from '@/modules/oneterm/api/account'
import {
  getPAMCapabilities,
  getPAMAccount,
  getPAMAccountBindings,
  getPAMPolicy,
  adoptPAMAccount,
  attachPAMAccount,
  reviewPAMBinding,
  setPAMAccountEnabled,
  savePAMPolicy,
  removeAccountManagement
} from '@/modules/oneterm/api/pam'
import ManagedAccountEditor from './managedAccountEditor.vue'
import PolicyEditor from './policyEditor.vue'
import MFAModal from '@/views/mfa/mfaModal'
import GrantModal from '@/modules/oneterm/components/grant/grantModal.vue'
import PasswordOperations from './passwordOperations.vue'

export default {
  name: 'AccountPasswordManagement',
  components: { ManagedAccountEditor, PolicyEditor, MFAModal, GrantModal, PasswordOperations },
  data() {
    return { visible: false,
      capabilities: null,
      detailGeneration: 0,
      bindingGeneration: 0,
      opening: 0,
      detailVisible: false,
      detailLoading: false,
      account: null,
      bindings: [],
      bindingTotal: 0,
      bindingPage: 1,
      policy: null,
      detailTab: 'bindings',
      pending: null,
      actionId: 0,
      retried: false,
      credentialVisible: false,
      credentialText: '' }
  },
  computed: {
    passwordPlatform() { return (this.capabilities?.managed_account_platforms || []).some((item) => item.id === this.account?.platform && (item.operations || []).includes('verify_secret')) },
    ...mapState({ windowHeight: (state) => state.windowHeight, roles: (state) => state.user.roles }),
    tableHeight() { return Math.max(220, this.windowHeight - 240) },
    isAdmin() { return (this.roles?.permissions || []).some((role) => ['admin', 'oneterm_admin', 'acl_admin'].includes(role)) }
  },
  beforeDestroy() { this.close() },
  methods: {
    async open(source) {
      const opening = ++this.opening
      this.visible = true
      this.capabilities = null
      if (!source) return
      await this.loadCapabilities()
      if (opening !== this.opening || !this.visible || !this.capabilities) return
      if (source.managed) await this.inspect(source.id)
      else this.adopt(source)
    },
    async loadCapabilities() {
      const opening = this.opening
      try {
        const response = await getPAMCapabilities()
        if (opening !== this.opening || !this.visible) return
        this.capabilities = response.data
      } catch (error) {
        if (opening === this.opening && this.visible) this.$message.error(this.$t('oneterm.pam.loadFailed'))
      }
    },
    close() { this.visible = false; this.opening += 1; this.closeDetail(); if (this.$refs.editor) this.$refs.editor.close() },
    closeDetail() { this.detailVisible = false; this.detailGeneration += 1; this.bindingGeneration += 1; this.account = null; this.policy = null; this.clearCredential(); this.cancelOperation() },
    clearCredential() { this.credentialVisible = false; this.credentialText = '' },
    async showCredential() {
      if (!this.account || !this.allowed(this.account, 'read')) return
      if (this.credentialVisible) {
        this.clearCredential()
        return
      }
      try {
        const response = await getAccountByCredentials(this.account.id)
        const data = response?.data || {}
        this.credentialText = Number(data.account_type) === 1 ? (data.password || '') : (data.pk || '')
        this.credentialVisible = true
      } catch (error) {
        this.clearCredential()
        this.$message.error(error?.response?.data?.message || this.$t('requestError'))
      }
    },
    copyCredential() {
      this.$copyText(this.credentialText).then(() => this.$message.success(this.$t('copySuccess')))
    },
    allowed(row, permission) { return this.isAdmin || (row?.permissions || []).includes(permission) },
    authority(row) { return row.authority_kind === 'shared' ? row.authority_ref : this.$t('oneterm.pam.localAccount') },
    async inspect(id, retainTab = false) {
      const generation = ++this.detailGeneration
      this.clearCredential()
      if (!retainTab) { this.account = null; this.policy = null; this.bindings = []; this.bindingTotal = 0; this.detailTab = 'bindings' }
      this.detailVisible = true; this.detailLoading = true
      try {
        const response = await getPAMAccount(id)
        if (generation !== this.detailGeneration || !this.detailVisible) return
        this.account = response.data
        await this.loadBindings(1)
        const policy = await getPAMPolicy(id)
        if (generation === this.detailGeneration && this.detailVisible) this.policy = policy.data
      } catch (error) {
        this.$message.error(this.$t('oneterm.pam.loadFailed'))
      } finally { if (generation === this.detailGeneration) this.detailLoading = false }
    },
    async loadBindings(page = 1) {
      if (!this.account) return
      const generation = ++this.bindingGeneration
      try {
        const response = await getPAMAccountBindings(this.account.id, { page_index: page, page_size: 50 })
        if (generation !== this.bindingGeneration || !this.detailVisible) return
        this.bindings = response.data?.list || []; this.bindingTotal = response.data?.count || 0; this.bindingPage = page
      } catch (error) {
        // Keep the last readable binding list on transient failures.
      }
    },
    adopt(source) { if (this.capabilities) this.$refs.editor.open(source || null, this.capabilities) },
    attach() { this.$refs.editor.open(this.account, this.capabilities, this.account) },
    grant(row) { this.$refs.grant.open({ type: 'account', resourceId: row.resource_id, ids: [row.id] }) },
    toggleAccount(row) {
      this.$confirm({ title: this.$t(row.enabled ? 'oneterm.pam.disableManagedAccount' : 'oneterm.pam.enableManagedAccount'),
        content: this.$t(row.enabled ? 'oneterm.pam.disableManagedConfirm' : 'oneterm.pam.enableManagedConfirm', { name: row.name }),
        onOk: () => this.begin({ type: 'enabled', id: row.id, revision: row.revision, enabled: !row.enabled }) })
    },
    removeManagement() {
      const { id, revision, name } = this.account
      this.$confirm({ title: this.$t('oneterm.pam.removeManagement'),
        content: this.$t('oneterm.pam.removeManagementConfirm', { name }),
        onOk: () => this.begin({ type: 'remove', id, revision }) })
    },
    review(row, enabled) {
      const id = this.account.id
      this.$confirm({ title: this.$t('oneterm.pam.reviewBinding'),
        content: this.$t('oneterm.pam.reviewBindingConfirm', { before: row.target_host, after: row.current_host || row.target_host }),
        onOk: () => this.begin({ type: 'binding', id, data: { asset_id: row.asset_id, account_id: row.account_id, revision: row.revision, enabled } }) })
    },
    savePolicy(data) {
      const id = this.account.id
      this.$confirm({ title: this.$t('oneterm.pam.savePolicy'),
        content: this.$t('oneterm.pam.policyChangeConfirm'),
        onOk: () => this.begin({ type: 'policy', id, data }) })
    },
    async begin(operation) {
      if (this.pending || !this.capabilities) return
      this.pending = operation; this.retried = false
      const action = ++this.actionId
      try { await this.$refs.mfa.open({ scope: this.capabilities.policy_mfa_scope, action }) } catch (error) { if (action === this.actionId) this.cancelOperation() }
    },
    cancelOperation() {
      this.actionId += 1; this.pending = null
      if (this.$refs.mfa) this.$refs.mfa.closeModal()
      if (this.$refs.editor) this.$refs.editor.finish(false)
    },
    async verified({ token, action }) {
      if (action !== this.actionId || !this.pending) return
      const operation = this.pending
      try {
        let response
        if (operation.type === 'adopt') response = await adoptPAMAccount(operation.data, token)
        else if (operation.type === 'attach') response = await attachPAMAccount(operation.id, operation.data, token)
        else if (operation.type === 'binding') response = await reviewPAMBinding(operation.id, operation.data, token)
        else if (operation.type === 'policy') response = await savePAMPolicy(operation.id, operation.data, token)
        else if (operation.type === 'remove') response = await removeAccountManagement(operation.id, operation.revision, token)
        else response = await setPAMAccountEnabled(operation.id, operation.revision, operation.enabled, token)
        if (action !== this.actionId) return
        this.pending = null
        this.$refs.editor.finish(true)
        this.$emit('updated')
        if (operation.type === 'remove') { this.close(); return }
        this.inspect(operation.type === 'adopt' ? response.data.id : operation.id, operation.type === 'policy')
      } catch (error) {
        if (action !== this.actionId) return
        if (Number(error?.response?.data?.code) === 4403 && !this.retried) {
          this.retried = true
          try { await this.$refs.mfa.open({ scope: this.capabilities.policy_mfa_scope, action, force: true }) } catch (challengeError) { if (action === this.actionId) this.cancelOperation() }
          return
        }
        this.$message.error(error?.response?.data?.message || this.$t('requestError'))
        this.cancelOperation()
      }
    }
  }
}
</script>

<style scoped>
.pam-managed-toolbar { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 12px; margin-bottom: 16px; }
.pam-managed-pagination { margin-top: 16px; text-align: right; }
.pam-managed-summary { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; margin-bottom: 16px; overflow-wrap: anywhere; }
.pam-managed-summary strong { font-size: 16px; }
.pam-managed-summary span { color: #78818b; }
.pam-managed-detail-actions { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }
.pam-credential-view { display: inline-flex; align-items: center; gap: 8px; max-width: 420px; }
.pam-credential-view span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pam-managed-warning { color: #a35b15; }
.pam-managed-detail-spin { height: 100%; }
.pam-managed-detail-spin /deep/ .ant-spin-container { display: flex; flex-direction: column; height: 100%; }
.pam-managed-summary, .pam-managed-detail-actions { flex-shrink: 0; }
.pam-managed-tabs { flex: 1; min-height: 0; display: flex; flex-direction: column; }
.pam-managed-tabs /deep/ .ant-tabs-bar { flex-shrink: 0; }
.pam-managed-tabs /deep/ .ant-tabs-content { flex: 1; min-height: 0; overflow: hidden; }
.pam-managed-tabs /deep/ .ant-tabs-tabpane-active { height: 100%; overflow: auto; }
</style>
