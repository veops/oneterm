<template>
  <div class="pam-password">
    <div class="pam-password-toolbar">
      <a-select
        v-model="bindingId"
        show-search
        :filter-option="false"
        :loading="bindingLoading"
        :disabled="busy"
        class="pam-password-target"
        :placeholder="$t('oneterm.asset')"
        @search="searchBindings"
        @change="selectBinding">
        <a-select-option v-for="item in bindings" :key="item.id">{{ item.asset_name || item.target_host }} · {{ item.target_host }}</a-select-option>
      </a-select>
      <div class="pam-password-actions">
        <a-tooltip :title="$t('oneterm.pam.password.connectionSettings')"><a-button :disabled="!selected || !canManage || busy" @click="openSettings"><a-icon type="setting" /></a-button></a-tooltip>
        <a-button :disabled="!ready || !allowed('verify_secret') || busy" :loading="verifying" @click="verify"><a-icon v-if="!verifying" type="check-circle" />{{ $t('oneterm.pam.password.verify') }}</a-button>
        <a-button type="primary" :disabled="!ready || account.account_type !== 1 || !allowed('rotate_secret') || busy || hasActiveOperation" @click="openChange('rotate_secret')"><a-icon type="sync" />{{ $t('oneterm.pam.password.change') }}</a-button>
        <a-button v-if="canReset" :disabled="busy || hasActiveOperation" @click="openChange('recover_secret')"><a-icon type="rollback" />{{ $t('oneterm.pam.password.actions.recover_secret') }}</a-button>
      </div>
    </div>
    <a-alert v-if="selected && selected.gateway_id" type="warning" show-icon :message="$t('oneterm.pam.password.gatewayUnavailable')" />
    <a-alert v-else-if="selected && selected.route_changed" type="warning" show-icon :message="$t('oneterm.pam.bindingNeedsReview')" />
    <a-alert v-else-if="selected && !ready" type="info" show-icon :message="$t('oneterm.pam.password.configurationRequired')" />
    <a-alert v-if="verification" :type="verification === 'verified' ? 'success' : 'warning'" show-icon :message="resultText(verification)" />
    <div class="pam-password-history-title">
      <strong>{{ $t('oneterm.pam.password.history') }}</strong>
      <a-tooltip :title="$t('refresh')"><a-button type="link" :loading="historyLoading" @click="loadHistory"><ops-icon v-if="!historyLoading" type="veops-refresh" /></a-button></a-tooltip>
    </div>
    <ops-table
      :data="rows"
      :height="320"
      :row-config="{ keyField: 'id' }"
      :scroll-y="{ enabled: true, gt: 20 }"
      stripe
      resizable
      show-overflow
      show-header-overflow
      size="small">
      <vxe-column :title="$t('oneterm.pam.password.createdAt')" min-width="165"><template #default="{ row }">{{ formatTime(row.created_at) }}</template></vxe-column>
      <vxe-column :title="$t('operation')" width="110"><template #default="{ row }">{{ $t('oneterm.pam.password.actions.' + row.action) }}</template></vxe-column>
      <vxe-column :title="$t('status')" min-width="120"><template #default="{ row }"><span :class="'pam-password-state-' + row.state">{{ $t('oneterm.pam.password.states.' + row.state) }}</span></template></vxe-column>
      <vxe-column :title="$t('oneterm.pam.password.result')" min-width="180"><template #default="{ row }">{{ row.error_code ? resultText(row.error_code) : '' }}</template></vxe-column>
      <vxe-column :title="$t('operation')" width="152" fixed="right"><template #default="{ row }">
        <a v-if="row.can_resume" :disabled="busy" @click="continueOperation(row, 'resume')">{{ $t('oneterm.pam.password.resume') }}</a>
        <a v-else-if="row.can_reconcile" :disabled="busy" @click="continueOperation(row, 'reconcile')">{{ $t('oneterm.pam.password.reconcile') }}</a>
        <a-dropdown v-if="row.can_reconcile" :trigger="['click']" :disabled="busy">
          <a-button type="link" size="small" :title="$t('operation')"><a-icon type="ellipsis" /></a-button>
          <a-menu slot="overlay"><a-menu-item @click="openOriginal(row)">{{ $t('oneterm.pam.password.keepOriginal') }}</a-menu-item></a-menu>
        </a-dropdown>
      </template></vxe-column>
    </ops-table>
    <div class="pam-password-pagination"><a-pagination size="small" :current="page" :page-size="50" :total="total" @change="changePage" /></div>

    <a-modal
      :visible="settingsVisible"
      :title="$t('oneterm.pam.password.connectionSettings')"
      :width="640"
      :confirm-loading="busy"
      :mask-closable="false"
      @ok="saveSettings"
      @cancel="closeSettings">
      <a-form-model ref="settingsForm" :model="form" layout="vertical">
        <a-form-model-item :label="$t('oneterm.protocol')" required><a-select v-model="form.protocol" :disabled="busy"><a-select-option v-for="protocol in protocols" :key="protocol">{{ protocol }}</a-select-option></a-select></a-form-model-item>
        <a-form-model-item v-if="platform.protocol === 'ssh'" :label="$t('oneterm.pam.password.hostKey')" required><a-input v-model="form.ssh_host_key" :disabled="busy" placeholder="SHA256:..." /></a-form-model-item>
        <template v-if="platform.protocol === 'mysql'">
          <a-form-model-item :label="$t('oneterm.pam.password.tlsServerName')"><a-input v-model="form.tls_server_name" :disabled="busy" :placeholder="selected ? selected.target_host : ''" /></a-form-model-item>
          <a-form-model-item :label="$t('oneterm.pam.password.caCertificate')"><a-textarea v-model="form.tls_ca" :rows="4" :disabled="busy" :placeholder="$t('oneterm.pam.password.systemCA')" /></a-form-model-item>
        </template>
        <a-form-model-item :label="$t('oneterm.pam.password.executionAccount')">
          <a-select
            v-model="form.executor_account_id"
            show-search
            :filter-option="false"
            :loading="executorLoading"
            :disabled="busy"
            @search="searchExecutors">
            <a-select-option :key="0">{{ $t('oneterm.pam.password.currentAccount') }} ({{ account.account }})</a-select-option>
            <a-select-option v-for="item in executors" :key="item.id" :disabled="!actorAllowed(item)">{{ item.name }} ({{ item.account }})</a-select-option>
            <a-select-option v-if="missingExecutor" :key="form.executor_account_id" disabled>{{ $t('oneterm.pam.password.unavailableExecutor') }}</a-select-option>
          </a-select>
        </a-form-model-item>
        <a-form-model-item v-if="platform.protocol === 'ssh'" label="sudo"><a-switch v-model="form.use_sudo" :disabled="busy" /></a-form-model-item>
      </a-form-model>
    </a-modal>

    <a-modal
      :visible="changeVisible"
      :title="$t('oneterm.pam.password.actions.' + changeAction)"
      :width="520"
      :confirm-loading="busy"
      :mask-closable="false"
      :ok-text="$t('oneterm.pam.password.confirmChange')"
      @ok="changePassword"
      @cancel="closeChange">
      <p class="pam-password-change-target"><strong>{{ account.account }}</strong><span>{{ selected ? selected.target_host : '' }}</span></p>
      <a-form-model layout="vertical"><a-form-model-item :label="$t('oneterm.pam.password.newPassword')"><a-input-password v-model="newPassword" autocomplete="new-password" :max-length="1024" :disabled="busy" :placeholder="$t('oneterm.pam.password.generatePassword')" /></a-form-model-item></a-form-model>
    </a-modal>
    <a-modal
      :visible="originalVisible"
      :title="$t('oneterm.pam.password.keepOriginal')"
      :width="540"
      :mask-closable="false"
      :confirm-loading="busy"
      :ok-button-props="{ props: { disabled: !remoteStopped || busy } }"
      @ok="keepOriginal"
      @cancel="closeOriginal">
      <p>{{ $t('oneterm.pam.password.originalConfirm') }}</p>
      <a-checkbox v-model="remoteStopped" :disabled="busy">{{ $t('oneterm.pam.password.remoteStopped') }}</a-checkbox>
    </a-modal>
    <MFAModal ref="mfa" @ok="verified" @cancel="cancelPending" />
  </div>
</template>

<script>
import moment from 'moment'
import { v4 as uuidv4 } from 'uuid'
import MFAModal from '@/views/mfa/mfaModal'
import {
  getPAMAccountBindings, getPAMPasswordExecutors, getPAMPasswordExecutions, savePAMPasswordConfig,
  verifyPAMPassword, startPAMPasswordChange, continuePAMPassword
} from '@/modules/oneterm/api/pam'

export default {
  name: 'PAMPasswordOperations',
  components: { MFAModal },
  props: { account: { type: Object, required: true }, capabilities: { type: Object, required: true }, isAdmin: Boolean },
  data() {
    return {
      bindingId: undefined,
      bindings: [],
      bindingLoading: false,
      bindingGeneration: 0,
      bindingTimer: null,
      rows: [],
      page: 1,
      total: 0,
      historyLoading: false,
      historyReady: false,
      timer: null,
      alive: true,
      verification: '',
      verifying: false,
      pending: null,
      actionId: 0,
      busy: false,
      settingsVisible: false,
      form: {},
      executors: [],
      executorLoading: false,
      executorGeneration: 0,
      executorTimer: null,
      changeVisible: false,
      changeAction: 'rotate_secret',
      newPassword: '',
      idempotencyKey: '',
      intent: '',
      originalVisible: false,
      originalRow: null,
      remoteStopped: false
    }
  },
  computed: {
    selected() { return this.bindings.find((item) => item.id === this.bindingId) },
    platform() { return (this.capabilities.managed_account_platforms || []).find((item) => item.id === this.account.platform) || {} },
    protocols() { return (this.selected?.protocols || []).filter((value) => value.split(':')[0] === this.platform.protocol) },
    canManage() { return this.allowed('manage_policy') && this.allowed('grant') },
    canReset() { return this.ready && this.account.account_type === 1 && this.allowed('recover_secret') && this.selected.password_config.executor_account_id > 0 && this.selected.password_config.executor_account_id !== this.account.id },
    ready() { return Boolean(this.selected?.enabled && !this.selected.route_changed && !this.selected.gateway_id && this.selected.password_config?.protocol) },
    hasActiveOperation() { return this.rows.some((row) => !['completed', 'failed'].includes(row.state)) },
    missingExecutor() { return Boolean(this.form.executor_account_id && !this.executors.some((item) => item.id === this.form.executor_account_id)) }
  },
  mounted() { this.loadBindings(''); this.loadHistory() },
  beforeDestroy() {
    this.alive = false; this.actionId += 1; this.bindingGeneration += 1; this.executorGeneration += 1
    clearTimeout(this.timer); clearTimeout(this.bindingTimer); clearTimeout(this.executorTimer)
    this.newPassword = ''; this.intent = ''; this.pending = null
  },
  methods: {
    allowed(action) { return this.isAdmin || (this.account.permissions || []).includes(action) },
    actorAllowed(actor) { return this.isAdmin || ['manage_policy', 'grant'].every((action) => (actor.permissions || []).includes(action)) },
    formatTime(value) { return value ? moment(value).format('YYYY-MM-DD HH:mm:ss') : '' },
    resultText(code) {
      const key = 'oneterm.pam.password.results.' + code
      return this.$te(key) ? this.$t(key) : this.$t('oneterm.pam.password.results.result_unknown')
    },
    report(error) { this.$message.error(error?.response?.data?.message || this.$t('requestError')) },
    searchBindings(value) { clearTimeout(this.bindingTimer); this.bindingTimer = setTimeout(() => this.loadBindings(value), 200) },
    async loadBindings(search) {
      const generation = ++this.bindingGeneration
      this.bindingLoading = true
      try {
        const response = await getPAMAccountBindings(this.account.id, { search, page_size: 50, page_index: 1 })
        if (!this.alive || generation !== this.bindingGeneration) return
        const previous = this.selected
        const rows = response.data?.list || []
        this.bindings = previous && !rows.some((row) => row.id === previous.id) ? [previous, ...rows] : rows
        if (!this.bindingId) this.bindingId = (rows.find((row) => row.enabled) || rows[0])?.id
      } catch (error) { if (this.alive) this.report(error) } finally { if (generation === this.bindingGeneration) this.bindingLoading = false }
    },
    selectBinding() { this.verification = '' },
    async loadHistory() {
      clearTimeout(this.timer)
      if (this.historyLoading || !this.alive) return
      this.historyLoading = true
      const page = this.page
      try {
        const response = await getPAMPasswordExecutions(this.account.id, { page_index: this.page, page_size: 50 })
        if (!this.alive || page !== this.page) return
        const rows = response.data?.list || []
        const changed = this.historyReady && rows.some((row) => row.state === 'completed' && !this.rows.some((old) => old.id === row.id && old.state === 'completed'))
        this.rows = rows; this.total = response.data?.count || 0; this.historyReady = true
        if (changed) this.$emit('updated')
      } catch (error) { if (this.alive && !this.historyReady) this.report(error) } finally {
        this.historyLoading = false
        if (this.alive && page !== this.page) {
          this.loadHistory()
        } else { this.scheduleHistory() }
      }
    },
    scheduleHistory() {
      if (this.alive) {
        this.timer = setTimeout(() => {
          if (document.visibilityState === 'hidden') this.scheduleHistory()
          else this.loadHistory()
        }, 5000)
      }
    },
    changePage(page) { this.page = page; this.loadHistory() },
    async openSettings() {
      if (!this.selected || !this.canManage || this.busy) return
      this.form = { protocol: this.protocols[0] || '', ssh_host_key: '', tls_ca: '', tls_server_name: '', executor_account_id: 0, use_sudo: false, ...(this.selected.password_config || {}) }
      this.settingsVisible = true
      this.loadExecutors('')
    },
    closeSettings() { if (!this.busy) this.settingsVisible = false },
    searchExecutors(value) { clearTimeout(this.executorTimer); this.executorTimer = setTimeout(() => this.loadExecutors(value), 200) },
    async loadExecutors(search) {
      const generation = ++this.executorGeneration
      this.executorLoading = true
      try {
        const response = await getPAMPasswordExecutors(this.account.id, { binding_id: this.bindingId, search, page_size: 50 })
        if (this.alive && generation === this.executorGeneration) this.executors = (response.data?.list || []).filter((item) => item.id !== this.account.id)
      } catch (error) { if (this.alive) this.report(error) } finally { if (generation === this.executorGeneration) this.executorLoading = false }
    },
    saveSettings() {
      if (!this.form.protocol || (this.platform.protocol === 'ssh' && !/^SHA256:[A-Za-z0-9+/]{43}$/.test(this.form.ssh_host_key))) {
        this.$message.error(this.$t('oneterm.pam.password.invalidConfiguration')); return
      }
      this.begin({ kind: 'settings', bindingId: this.bindingId, data: { ...this.form, revision: this.selected.revision } })
    },
    async verify() {
      if (!this.ready || this.busy) return
      this.busy = true; this.verifying = true
      try {
        const response = await verifyPAMPassword(this.account.id, { binding_id: this.bindingId, binding_revision: this.selected.revision, credential_revision: this.account.credential_revision })
        if (this.alive) this.verification = response.data.code
      } catch (error) { if (this.alive) { this.report(error); this.$emit('updated') } } finally { this.busy = false; this.verifying = false }
    },
    openChange(action = 'rotate_secret') {
      if (this.ready && !this.busy && this.allowed(action)) {
        this.changeAction = action; this.newPassword = ''; this.idempotencyKey = ''; this.intent = ''; this.changeVisible = true
      }
    },
    closeChange() { if (!this.busy) { this.changeVisible = false; this.newPassword = ''; this.intent = '' } },
    changePassword() {
      if (!this.selected || this.busy) return
      const intent = JSON.stringify([this.bindingId, this.selected.revision, this.account.credential_revision, this.changeAction, this.newPassword])
      if (intent !== this.intent) { this.intent = intent; this.idempotencyKey = uuidv4() }
      this.begin({ kind: 'change',
        data: { binding_id: this.bindingId,
          binding_revision: this.selected.revision,
          credential_revision: this.account.credential_revision,
          password: this.newPassword,
          action: this.changeAction,
          idempotency_key: this.idempotencyKey } })
    },
    continueOperation(row, operation) {
      if (this.busy) return
      this.$confirm({ title: this.$t('oneterm.pam.password.' + (operation === 'resume' ? 'resume' : 'reconcile')),
        content: this.$t('oneterm.pam.password.' + (operation === 'resume' ? 'resumeConfirm' : 'reconcileConfirm')),
        onOk: () => this.begin({ kind: operation, executionId: row.id }) })
    },
    openOriginal(row) { if (!this.busy && row.can_reconcile) { this.originalRow = row; this.remoteStopped = false; this.originalVisible = true } },
    closeOriginal() { if (!this.busy) { this.originalVisible = false; this.originalRow = null; this.remoteStopped = false } },
    keepOriginal() {
      if (!this.remoteStopped || !this.originalRow || this.busy) return
      this.begin({ kind: 'keep-original', executionId: this.originalRow.id, data: { revision: this.originalRow.revision, remote_stopped: true } })
    },
    async begin(operation) {
      if (this.busy || this.pending) return
      const action = ++this.actionId
      this.pending = operation
      try { await this.$refs.mfa.open({ scope: operation.kind === 'settings' ? this.capabilities.policy_mfa_scope : 'oneterm_pam_rotate', action }) } catch (error) { if (action === this.actionId) this.cancelPending() }
    },
    cancelPending() { if (!this.busy) { this.actionId += 1; this.pending = null } },
    async verified({ token, action }) {
      if (!this.alive || action !== this.actionId || !this.pending || this.busy) return
      const operation = this.pending
      let rechallenged = false
      this.busy = true
      try {
        let response
        if (operation.kind === 'settings') response = await savePAMPasswordConfig(this.account.id, operation.bindingId, operation.data, token)
        else if (operation.kind === 'change') response = await startPAMPasswordChange(this.account.id, operation.data, token)
        else if (operation.kind === 'keep-original') response = await continuePAMPassword(this.account.id, operation.executionId, operation.kind, token, operation.data)
        else response = await continuePAMPassword(this.account.id, operation.executionId, operation.kind, token)
        if (!this.alive || action !== this.actionId) return
        if (operation.kind === 'settings') {
          const saved = response.data
          this.bindings = this.bindings.map((item) => item.id === operation.bindingId ? { ...item, revision: saved.revision, password_config: saved.password_config } : item)
          this.settingsVisible = false
          await this.loadBindings('')
        } else { this.changeVisible = false; this.originalVisible = false; this.newPassword = ''; this.intent = ''; await this.loadHistory() }
        this.$emit('updated')
        if (operation.kind === 'settings' || response.data.state === 'completed' || response.data.error_code === 'original_verified') this.$message.success(this.$t('saveSuccess'))
      } catch (error) {
        if (this.alive && action === this.actionId && Number(error?.response?.data?.code) === 4403 && !operation.retried) {
          operation.retried = true; rechallenged = true; this.busy = false
          try { await this.$refs.mfa.open({ scope: operation.kind === 'settings' ? this.capabilities.policy_mfa_scope : 'oneterm_pam_rotate', action, force: true }) } catch (challengeError) { this.cancelPending() }
          return
        }
        if (this.alive) { this.report(error); await this.loadHistory() }
      } finally {
        if (!rechallenged) {
          this.busy = false; this.pending = null
          if (operation.data && Object.prototype.hasOwnProperty.call(operation.data, 'password')) operation.data.password = ''
        }
      }
    }
  }
}
</script>

<style scoped>
.pam-password-toolbar { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.pam-password-target { width: 280px; max-width: 100%; }
.pam-password-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.pam-password /deep/ .ant-alert { margin-bottom: 12px; }
.pam-password-history-title { display: flex; justify-content: space-between; align-items: center; margin: 16px 0 8px; }
.pam-password-pagination { margin-top: 12px; text-align: right; }
.pam-password-change-target { display: flex; flex-wrap: wrap; gap: 12px; overflow-wrap: anywhere; }
.pam-password-change-target span { color: #78818b; }
.pam-password-state-completed { color: #247445; }
.pam-password-state-unknown, .pam-password-state-failed { color: #b33d35; }
@media (max-width: 640px) { .pam-password-target { width: 100%; } }
</style>
