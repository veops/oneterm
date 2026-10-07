<template>
  <div class="oneterm-layout">
    <div class="oneterm-header">{{ $t('oneterm.pam.audit') }}</div>
    <div class="oneterm-layout-container">
      <div class="oneterm-layout-container-header pam-audit-toolbar">
        <a-space>
          <a-select v-model="outcome" :style="{ width: '140px' }" @change="load(1)">
            <a-select-option value="">{{ $t('oneterm.pam.allOutcomes') }}</a-select-option>
            <a-select-option v-for="value in outcomes" :key="value" :value="value">{{ outcomeLabel(value) }}</a-select-option>
          </a-select>
          <a-range-picker v-model="range" show-time format="YYYY-MM-DD HH:mm" @change="load(1)" />
          <a-tag v-if="applicationId" closable @close="clearApplication">{{ $t('oneterm.pam.applicationAudit', { name: applicationName }) }}</a-tag>
        </a-space>
        <a-space>
          <a-tooltip :title="$t('refresh')">
            <a-button :loading="loading" @click="load(page)"><ops-icon v-if="!loading" type="veops-refresh" /></a-button>
          </a-tooltip>
          <a-dropdown :disabled="!rows.length">
            <a-button><a-icon type="download" />{{ $t('oneterm.pam.exportPage') }}</a-button>
            <a-menu slot="overlay" @click="({ key }) => exportPage(key)">
              <a-menu-item key="csv">CSV</a-menu-item>
              <a-menu-item key="xlsx">Excel</a-menu-item>
            </a-menu>
          </a-dropdown>
        </a-space>
      </div>
      <ops-table
        ref="table"
        :data="displayRows"
        :height="tableHeight"
        :row-config="{ keyField: 'id' }"
        :scroll-y="{ enabled: true, gt: 20 }"
        size="small"
        stripe
        resizable
        show-overflow
        show-header-overflow
        class="ops-stripe-table"
      >
        <vxe-column field="time_label" :title="$t('created_at')" width="174" />
        <vxe-column field="actor_label" :title="$t('oneterm.pam.actor')" min-width="180" />
        <vxe-column field="target_label" :title="$t('oneterm.pam.credentialTarget')" min-width="160" />
        <vxe-column field="action_label" :title="$t('oneterm.pam.action')" min-width="140" />
        <vxe-column field="outcome_label" :title="$t('oneterm.pam.outcome')" width="110" />
        <vxe-column field="reason_label" :title="$t('oneterm.pam.reason')" min-width="160" />
        <vxe-column field="source_ip" :title="$t('oneterm.pam.sourceIP')" width="155" />
        <vxe-column field="request_id" :title="$t('oneterm.pam.requestID')" min-width="220" />
        <vxe-column field="version_id" :title="$t('oneterm.pam.versionID')" min-width="220" />
      </ops-table>
      <div class="oneterm-layout-pagination">
        <a-pagination
          size="small"
          show-size-changer
          :current="page"
          :page-size="pageSize"
          :total="total"
          @change="load"
          @showSizeChange="load" />
      </div>
    </div>
  </div>
</template>

<script>
import moment from 'moment'
import { mapState } from 'vuex'
import { getPAMAudit, getPAMSubjects } from '@/modules/oneterm/api/pam'
import { ACTION_LABELS } from '@/modules/oneterm/components/grant/permissions'

export default {
  name: 'PAMAccessAudit',
  data() {
    return {
      rows: [],
      subjects: {},
      loading: false,
      generation: 0,
      outcome: '',
      page: 1,
      pageSize: 20,
      total: 0,
      range: [],
      outcomes: ['succeeded', 'denied', 'failed', 'pending']
    }
  },
  computed: {
    ...mapState({ windowHeight: (state) => state.windowHeight }),
    tableHeight() { return Math.max(240, this.windowHeight - 258) },
    applicationId() { return this.$route.query.actor_type === 'application' ? this.$route.query.actor_id : '' },
    applicationName() { return this.$route.query.application_name || `#${this.applicationId}` },
    displayRows() {
      return this.rows.map((row) => {
        const subject = this.subjects[row.acl_uid]
        const identity = subject ? (subject.nickname || subject.username) : `#${row.acl_uid}`
        return {
          ...row,
          time_label: moment(row.created_at).format('YYYY-MM-DD HH:mm:ss'),
          actor_label: row.actor_type === 'system' ? this.$t('oneterm.pam.systemActor') : row.actor_type === 'application' ? this.$t('oneterm.pam.applicationActor', { id: row.actor_id, name: identity }) : identity,
          target_label: `${this.kindLabel(row.target_kind)} #${row.target_id}`,
          action_label: this.actionLabel(row.action),
          outcome_label: this.outcomeLabel(row.outcome),
          reason_label: this.reasonLabel(row.reason_code)
        }
      })
    }
  },
  watch: { '$route.query': { handler() { this.load(1) }, immediate: true } },
  beforeDestroy() { this.generation += 1 },
  methods: {
    actionLabel(value) {
      const labels = {
        ...ACTION_LABELS,
        reconcile_secret: 'oneterm.pam.password.reconcile',
        preserve_original_secret: 'oneterm.pam.password.keepOriginal',
        submitted: 'oneterm.pam.requestSubmitted',
        cancelled: 'oneterm.pam.withdrawRequest',
        expired: 'oneterm.pam.requestStates.expired',
        itsm_decision: 'oneterm.pam.approvalHistory',
        itsm_sync_retry: 'oneterm.pam.retrySync'
      }
      return labels[value] ? this.$t(labels[value]) : value
    },
    async load(page = 1, pageSize = this.pageSize) {
      const generation = ++this.generation
      this.loading = true
      try {
        const response = await getPAMAudit({
          view: this.applicationId ? undefined : 'all',
          page_index: page,
          page_size: pageSize,
          outcome: this.outcome || undefined,
          actor_type: this.applicationId ? 'application' : undefined,
          actor_id: this.applicationId || undefined,
          start: this.range?.[0]?.toISOString(),
          end: this.range?.[1]?.toISOString()
        })
        if (generation !== this.generation) return
        this.rows = response.data?.list || []
        this.total = response.data?.count || 0
        this.page = page
        this.pageSize = pageSize
        const uids = [...new Set(this.rows.map((row) => row.acl_uid).filter(Boolean))]
        if (uids.length) {
          const subjects = await getPAMSubjects({ uids: uids.join(','), page_size: 200 })
          if (generation === this.generation) this.subjects = Object.fromEntries((subjects.users || []).map((user) => [user.uid, user]))
        }
      } catch (error) {
        // Keep the last successful page; shared request handling reports failures.
      } finally {
        if (generation === this.generation) this.loading = false
      }
    },
    clearApplication() { this.$router.replace({ path: '/oneterm/pam/audit' }) },
    kindLabel(kind) {
      const labels = { account: 'oneterm.account', gateway: 'oneterm.gateway' }
      return labels[kind] ? this.$t(labels[kind]) : kind
    },
    outcomeLabel(value) { return this.outcomes.includes(value) ? this.$t(`oneterm.pam.outcomes.${value}`) : value },
    reasonLabel(value) {
      const passwordKey = `oneterm.pam.password.results.${value}`
      if (this.$te(passwordKey)) return this.$t(passwordKey)
      const known = ['not_permitted', 'evidence_replayed', 'storage_unavailable', 'mfa_required', 'approval_required']
      return known.includes(value) ? this.$t(`oneterm.pam.reasons.${value}`) : (value || '-')
    },
    exportPage(type) {
      const table = this.$refs.table.getVxetableRef()
      const data = this.displayRows.map((row) => Object.fromEntries(Object.entries(row).map(([key, value]) => {
        const unsafeCSV = type === 'csv' && typeof value === 'string' && /^[\s\uFEFF]*[=+@-]/.test(value)
        return [key, unsafeCSV ? `'${value}` : value]
      })))
      table.exportData({ type, filename: `pam-access-${moment().format('YYYYMMDD-HHmm')}`, data, isFooter: false })
    }
  }
}
</script>

<style lang="less" scoped>
@import '../../style/index.less';
.pam-audit-toolbar { flex-wrap: wrap; gap: 12px; }
</style>
