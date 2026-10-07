<template>
  <div class="oneterm-layout">
    <div class="oneterm-header">{{ $t('oneterm.passwordView.auditTitle') }}</div>
    <div class="oneterm-layout-container">
      <div class="password-audit-toolbar">
        <a-input-search
          v-model="search"
          allow-clear
          :placeholder="$t('oneterm.passwordView.searchAccounts')"
          @search="load(1)"
        />
        <a-select v-model="action" @change="load(1)">
          <a-select-option value="">{{ $t('oneterm.passwordView.allEvents') }}</a-select-option>
          <a-select-option v-for="item in actions" :key="item">{{ actionLabel({ action: item }) }}</a-select-option>
        </a-select>
        <a-range-picker v-model="range" show-time format="YYYY-MM-DD HH:mm" @change="load(1)" />
        <a-tooltip
          :title="$t('refresh')"
        ><a-button
          :loading="loading"
          :aria-label="$t('refresh')"
          @click="load(page)"
        ><ops-icon v-if="!loading" type="veops-refresh" /></a-button
        ></a-tooltip>
        <a-dropdown :disabled="!rows.length">
          <a-button><a-icon type="download" />{{ $t('oneterm.pam.exportPage') }}</a-button>
          <a-menu
            slot="overlay"
            @click="({ key }) => exportPage(key)"
          ><a-menu-item key="csv">CSV</a-menu-item><a-menu-item key="xlsx">Excel</a-menu-item></a-menu
          >
        </a-dropdown>
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
        <vxe-column field="target_name" :title="$t('oneterm.name')" min-width="160" />
        <vxe-column field="target_username" :title="$t('oneterm.account')" min-width="140" />
        <vxe-column field="requester_label" :title="$t('oneterm.passwordView.requester')" min-width="130" />
        <vxe-column field="actor_label" :title="$t('oneterm.pam.actor')" min-width="140" />
        <vxe-column field="action_label" :title="$t('oneterm.passwordView.event')" min-width="140" />
        <vxe-column field="outcome_label" :title="$t('oneterm.pam.outcome')" min-width="100" />
        <vxe-column field="source_ip" :title="$t('oneterm.pam.sourceIP')" min-width="130" />
        <vxe-column
          :title="$t('operation')"
          width="90"
          fixed="right"
        ><template
          #default="{ row }"
        ><a-tooltip
          :title="$t('detail')"
        ><a-button
          type="link"
          :aria-label="$t('detail')"
          @click="selected = row"
        ><a-icon type="profile" /></a-button></a-tooltip></template
        ></vxe-column>
      </ops-table>
      <div class="oneterm-layout-pagination">
        <a-pagination
          size="small"
          show-size-changer
          :current="page"
          :page-size="pageSize"
          :total="total"
          @change="load"
          @showSizeChange="load"
        />
      </div>
    </div>
    <a-drawer
      :visible="!!selected"
      width="min(620px, 100vw)"
      :title="$t('oneterm.passwordView.auditTitle')"
      @close="selected = null"
    >
      <a-descriptions v-if="selected" :column="1" size="small">
        <a-descriptions-item :label="$t('oneterm.name')">{{ selected.target_name }}</a-descriptions-item>
        <a-descriptions-item :label="$t('oneterm.account')">{{ selected.target_username }}</a-descriptions-item>
        <a-descriptions-item :label="$t('oneterm.passwordView.event')">{{ selected.action_label }}</a-descriptions-item>
        <a-descriptions-item :label="$t('created_at')">{{ selected.time_label }}</a-descriptions-item>
        <a-descriptions-item :label="$t('oneterm.passwordView.requester')">{{
          selected.requester_label
        }}</a-descriptions-item>
        <a-descriptions-item :label="$t('oneterm.pam.actor')">{{ selected.actor_label }}</a-descriptions-item>
        <a-descriptions-item :label="$t('oneterm.pam.outcome')">{{ selected.outcome_label }}</a-descriptions-item>
        <a-descriptions-item v-if="selected.reason_code" :label="$t('oneterm.pam.reason')">{{
          reasonLabel(selected.reason_code)
        }}</a-descriptions-item>
        <a-descriptions-item :label="$t('oneterm.pam.sourceIP')">{{ selected.source_ip || '-' }}</a-descriptions-item>
        <a-descriptions-item v-if="selected.request_reason" :label="$t('oneterm.pam.requestReason')">{{
          selected.request_reason
        }}</a-descriptions-item>
        <a-descriptions-item
          v-if="selected.approver_uids && selected.approver_uids.length"
          :label="$t('oneterm.passwordView.approvers')"
        >{{ selected.approver_uids.map(userName).join(', ') }}</a-descriptions-item
        >
        <a-descriptions-item v-if="selected.granted_at" :label="$t('oneterm.passwordView.validFrom')">{{
          formatTime(selected.granted_at)
        }}</a-descriptions-item>
        <a-descriptions-item v-if="selected.granted_until" :label="$t('oneterm.passwordView.validUntil')">{{
          formatTime(selected.granted_until)
        }}</a-descriptions-item>
        <a-descriptions-item v-if="selected.request_state" :label="$t('oneterm.passwordView.currentRequestState')">{{
          requestState(selected)
        }}</a-descriptions-item>
        <a-descriptions-item v-if="selected.grant_revoked_at" :label="$t('oneterm.passwordView.revokedAt')">{{
          formatTime(selected.grant_revoked_at)
        }}</a-descriptions-item>
      </a-descriptions>
    </a-drawer>
  </div>
</template>

<script>
import moment from 'moment'
import { mapState } from 'vuex'
import { getPasswordViewAudit, getPAMSubjects } from '@/modules/oneterm/api/pam'
import { passwordViewStateLabel } from '@/modules/oneterm/components/passwordViewState'

export default {
  name: 'PasswordViewAudit',
  data() {
    return {
      rows: [],
      users: {},
      search: '',
      action: '',
      range: [],
      page: 1,
      pageSize: 20,
      total: 0,
      loading: false,
      generation: 0,
      selected: null,
      actions: ['submitted', 'itsm_decision', 'retrieve', 'cancelled'],
    }
  },
  computed: {
    ...mapState({ windowHeight: (state) => state.windowHeight }),
    tableHeight() {
      return Math.max(240, this.windowHeight - 270)
    },
    displayRows() {
      return this.rows.map((row) => ({
        ...row,
        time_label: this.formatTime(row.created_at),
        requester_label: this.userName(row.requester_uid),
        actor_label: row.actor_type === 'system' ? this.$t('oneterm.pam.systemActor') : this.userName(row.acl_uid),
        action_label: this.actionLabel(row),
        outcome_label: this.$t('oneterm.pam.outcomes.' + row.outcome),
      }))
    },
  },
  mounted() {
    this.load(1)
  },
  beforeDestroy() {
    this.generation += 1
  },
  methods: {
    formatTime(value) {
      return moment(value).format('YYYY-MM-DD HH:mm:ss')
    },
    userName(uid) {
      const user = this.users[uid]
      return user ? user.nickname || user.username : uid ? '#' + uid : '-'
    },
    actionLabel(row) {
      if (row.action === 'itsm_decision' && row.event_state) { return this.$t('oneterm.passwordView.events.' + row.event_state) }
      const keys = {
        submitted: 'oneterm.pam.requestSubmitted',
        itsm_decision: 'oneterm.pam.approvalHistory',
        retrieve: 'oneterm.credential.viewLoginInfo',
        cancelled: 'oneterm.pam.withdrawRequest',
        itsm_sync_retry: 'oneterm.pam.retrySync',
        expired: 'oneterm.passwordView.states.expired',
      }
      return keys[row.action] ? this.$t(keys[row.action]) : row.action
    },
    requestState(row) {
      const state =
        row.request_state === 'approved' && row.granted_until && moment(row.granted_until).isBefore(moment())
          ? 'expired'
          : row.request_state
      return passwordViewStateLabel({ state }, this.$t.bind(this))
    },
    reasonLabel(code) {
      const key = 'oneterm.pam.reasons.' + code
      return this.$te(key) ? this.$t(key) : this.$t('requestError')
    },
    async load(page = 1, pageSize = this.pageSize) {
      const generation = ++this.generation
      this.loading = true
      try {
        const response = await getPasswordViewAudit({
          view: 'all',
          page_index: page,
          page_size: pageSize,
          search: this.search,
          action: this.action || undefined,
          start: this.range[0]?.toISOString(),
          end: this.range[1]?.toISOString(),
        })
        if (generation !== this.generation) return
        this.rows = response.data.list || []
        this.total = response.data.count || 0
        this.page = page
        this.pageSize = pageSize
        const ids = [
          ...new Set(
            this.rows.flatMap((row) => [row.acl_uid, row.requester_uid, ...(row.approver_uids || [])]).filter(Boolean)
          ),
        ]
        if (ids.length) {
          const subjects = await getPAMSubjects({ uids: ids.join(','), page_size: 200 })
          if (generation === this.generation) { this.users = Object.fromEntries((subjects.users || []).map((user) => [user.uid, user])) }
        }
      } catch (error) {
        if (generation === this.generation) this.$message.error(this.$t('oneterm.pam.loadFailed'))
      } finally {
        if (generation === this.generation) this.loading = false
      }
    },
    exportPage(type) {
      this.$refs.table.getVxetableRef().exportData({
        type,
        filename: 'password-view-audit',
        isHeader: true,
        data: this.displayRows,
        columns: this.$refs.table
          .getVxetableRef()
          .getColumns()
          .filter((column) => column.field),
      })
    },
  },
}
</script>

<style lang="less" scoped>
@import '../../style/index.less';
.password-audit-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}
.password-audit-toolbar .ant-input-search {
  width: 250px;
  max-width: 100%;
}
.password-audit-toolbar .ant-select {
  width: 160px;
}
</style>
