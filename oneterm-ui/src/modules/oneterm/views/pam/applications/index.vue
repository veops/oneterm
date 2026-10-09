<template>
  <div class="oneterm-layout">
    <div class="oneterm-header">{{ $t('oneterm.pam.applications') }}</div>
    <div class="oneterm-layout-container">
      <div class="oneterm-layout-container-header pam-toolbar">
        <a-space>
          <a-input-search v-model="search" allow-clear :placeholder="$t('placeholderSearch')" @search="load(1)" />
          <a-select v-model="enabledFilter" :style="{ width: '120px' }" @change="load(1)">
            <a-select-option value="">{{ $t('oneterm.pam.allStates') }}</a-select-option>
            <a-select-option value="true">{{ $t('oneterm.enabled') }}</a-select-option>
            <a-select-option value="false">{{ $t('oneterm.disabled') }}</a-select-option>
          </a-select>
        </a-space>
        <a-space>
          <a-tooltip :title="$t('refresh')">
            <a-button :loading="loading" :disabled="Boolean(pending)" @click="load(page)"><ops-icon v-if="!loading" type="veops-refresh" /></a-button>
          </a-tooltip>
          <a-button v-if="isAdmin" type="primary" :disabled="!capabilities || Boolean(pending)" @click="edit(null)">
            <a-icon type="plus" />{{ $t('oneterm.pam.createApplication') }}
          </a-button>
        </a-space>
      </div>
      <ops-table
        :data="rows"
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
        <vxe-column field="name" :title="$t('oneterm.name')" min-width="140" />
        <vxe-column field="acl_uid" :title="$t('oneterm.pam.serviceIdentity')" min-width="160">
          <template #default="{ row }">{{ subjectLabel(row.acl_uid) }}</template>
        </vxe-column>
        <vxe-column :title="$t('oneterm.pam.applicationStatus')" width="88">
          <template #default="{ row }"><span :class="'pam-state-' + state(row)">{{ stateLabel(row) }}</span></template>
        </vxe-column>
        <vxe-column :title="$t('oneterm.pam.credentialScope')" width="100">
          <template #default="{ row }">{{ $t('oneterm.pam.targetCount', { count: targetCount(row) }) }}</template>
        </vxe-column>
        <vxe-column field="purpose" :title="$t('oneterm.pam.purpose')" min-width="150" />
        <vxe-column :title="$t('oneterm.pam.sourceRange')" min-width="135">
          <template #default="{ row }">{{ (row.source_cidrs || []).join(', ') || $t('oneterm.pam.anySource') }}</template>
        </vxe-column>
        <vxe-column :title="$t('oneterm.pam.validUntil')" width="148">
          <template #default="{ row }">{{ row.expires_at ? date(row.expires_at) : $t('oneterm.pam.permanent') }}</template>
        </vxe-column>
        <vxe-column :title="$t('operation')" width="148" fixed="right">
          <template #default="{ row }">
            <a-space class="pam-row-actions">
              <a-tooltip :title="$t('edit')">
                <a-button type="link" size="small" :disabled="!allowed(row, 'write') || Boolean(pending)" @click="edit(row)"><ops-icon type="icon-xianxing-edit" /></a-button>
              </a-tooltip>
              <a-tooltip :title="$t('grant')">
                <a-button type="link" size="small" :disabled="!allowed(row, 'grant')" @click="grant(row)"><a-icon type="team" /></a-button>
              </a-tooltip>
              <a-tooltip :title="$t('oneterm.pam.disableApplication')">
                <a-button type="link" size="small" :disabled="!row.enabled || !allowed(row, 'write') || Boolean(pending)" @click="confirmDisable(row)"><a-icon type="poweroff" /></a-button>
              </a-tooltip>
            </a-space>
          </template>
        </vxe-column>
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
    <ApplicationEditor ref="editor" @submit="save" @cancel="cancelOperation" />
    <MFAModal ref="mfa" @ok="verified" @cancel="cancelOperation" />
    <GrantModal ref="grant" />
  </div>
</template>

<script>
import moment from 'moment'
import { mapState } from 'vuex'
import {
  getPAMApplications, getPAMCapabilities, getPAMSubjects,
  createPAMApplication, updatePAMApplication, disablePAMApplication
} from '@/modules/oneterm/api/pam'
import { getAllDepAndEmployee } from '@/api/company'
import MFAModal from '@/views/mfa/mfaModal'
import GrantModal from '@/modules/oneterm/components/grant/grantModal.vue'
import ApplicationEditor from './editor.vue'
import { applicationState } from './form'

export default {
  name: 'PAMApplications',
  components: { ApplicationEditor, MFAModal, GrantModal },
  provide() { return { provide_allTreeDepAndEmp: () => this.allTreeDepAndEmp } },
  data() {
    return {
      rows: [],
      subjects: {},
      search: '',
      enabledFilter: '',
      page: 1,
      pageSize: 20,
      total: 0,
      loading: false,
      generation: 0,
      now: Date.now(),
      clockTimer: null,
      capabilities: null,
      pending: null,
      actionId: 0,
      retried: false,
      allTreeDepAndEmp: [],
      allTreeLoaded: false
    }
  },
  computed: {
    ...mapState({ windowHeight: (state) => state.windowHeight, roles: (state) => state.user.roles }),
    tableHeight() { return Math.max(240, this.windowHeight - 258) },
    isAdmin() { return (this.roles?.permissions || []).some((role) => ['oneterm_admin', 'acl_admin'].includes(role)) }
  },
  mounted() {
    this.load()
    getPAMCapabilities().then((response) => { this.capabilities = response.data }).catch(() => {})
    this.clockTimer = setInterval(() => { this.now = Date.now() }, 60000)
  },
  beforeDestroy() {
    this.generation += 1
    this.actionId += 1
    clearInterval(this.clockTimer)
  },
  methods: {
    async load(page = 1, pageSize = this.pageSize) {
      const generation = ++this.generation
      this.loading = true
      try {
        const response = await getPAMApplications({ page_index: page, page_size: pageSize, search: this.search, enabled: this.enabledFilter })
        if (generation !== this.generation) return
        this.rows = response.data?.list || []
        this.total = response.data?.count || 0
        this.page = page
        this.pageSize = pageSize
        const uids = [...new Set(this.rows.map((row) => row.acl_uid))]
        if (uids.length) {
          const names = await getPAMSubjects({ uids: uids.join(','), page_size: 200 })
          if (generation !== this.generation) return
          this.subjects = Object.fromEntries((names.users || []).map((user) => [user.uid, user]))
        }
      } catch (error) {
        // Shared request handling displays the failure; keep the current rows in place.
      } finally {
        if (generation === this.generation) this.loading = false
      }
    },
    allowed(row, action) { return this.isAdmin || (row.permissions || []).includes(action) },
    state(row) { return applicationState(row, this.now) },
    stateLabel(row) {
      const state = this.state(row)
      return state === 'expired' ? this.$t('oneterm.pam.expired') : this.$t(`oneterm.${state}`)
    },
    targetCount(row) { return Object.values(row.targets || {}).reduce((count, ids) => count + ids.length, 0) },
    subjectLabel(uid) {
      const subject = this.subjects[uid]
      return subject ? `${subject.nickname || subject.username} (${subject.username})` : `#${uid}`
    },
    date(value) { return moment(value).format('YYYY-MM-DD HH:mm') },
    edit(row) { if (this.capabilities) this.$refs.editor.open(row, this.capabilities) },
    async grant(row) {
      if (!this.allTreeLoaded) {
        this.allTreeDepAndEmp = await getAllDepAndEmployee({ block: 0 })
        this.allTreeLoaded = true
      }
      this.$refs.grant.open({ type: 'pam_application', resourceId: row.resource_id, ids: [row.id] })
    },
    save(input) { this.begin({ type: 'save', ...input }) },
    confirmDisable(row) {
      this.$confirm({
        title: this.$t('oneterm.pam.disableApplication'),
        content: this.$t('oneterm.pam.disableConfirm', { name: row.name }),
        onOk: () => this.begin({ type: 'disable', id: row.id, revision: row.revision })
      })
    },
    async begin(operation) {
      if (this.pending) return
      if (!this.capabilities) { this.$refs.editor.finish(false); return }
      this.pending = operation
      this.retried = false
      const action = ++this.actionId
      try {
        await this.$refs.mfa.open({ scope: this.capabilities.policy_mfa_scope, action })
      } catch (error) {
        if (action === this.actionId) this.cancelOperation()
      }
    },
    cancelOperation() {
      this.actionId += 1
      this.pending = null
      if (this.$refs.editor) this.$refs.editor.finish(false)
      if (this.$refs.mfa) this.$refs.mfa.closeModal()
    },
    async verified({ token, action }) {
      if (action !== this.actionId || !this.pending) return
      const operation = this.pending
      try {
        if (operation.type === 'disable') await disablePAMApplication(operation.id, operation.revision, token)
        else if (operation.id) await updatePAMApplication(operation.id, operation.data, token)
        else await createPAMApplication(operation.data, token)
        if (action !== this.actionId) return
        this.pending = null
        this.$message.success(this.$t('operateSuccess'))
        this.$refs.editor.finish(true)
        this.load(this.page)
      } catch (error) {
        if (action !== this.actionId) return
        if (Number(error?.response?.data?.code) === 4403 && !this.retried) {
          this.retried = true
          try {
            await this.$refs.mfa.open({ scope: this.capabilities.policy_mfa_scope, action, force: true })
          } catch (challengeError) { if (action === this.actionId) this.cancelOperation() }
          return
        }
        this.$message.error(error?.response?.data?.message || this.$t('requestError'))
        this.cancelOperation()
      }
    }
  }
}
</script>

<style lang="less" scoped>
@import '../../../style/index.less';
.pam-toolbar { flex-wrap: wrap; gap: 12px; }
.pam-row-actions /deep/ .ant-btn { width: 24px; padding: 0; }
.pam-state-enabled { color: #247445; }
.pam-state-disabled { color: #777777; }
.pam-state-expired { color: #a55416; }
</style>
