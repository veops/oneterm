<template>
  <div class="workstation-passwords">
    <div class="workstation-passwords-toolbar">
      <a-input-search v-model="search" allow-clear :placeholder="$t('oneterm.passwordView.searchAccounts')" @search="load(1)" />
      <a-button v-if="accountId" type="link" @click="$emit('clear-target')">{{ $t('oneterm.passwordView.allAccounts') }}</a-button>
      <a-tooltip :title="$t('refresh')"><a-button :loading="loading" :aria-label="$t('refresh')" @click="load(page)"><ops-icon v-if="!loading" type="veops-refresh" /></a-button></a-tooltip>
    </div>
    <a-alert v-if="failed" type="error" show-icon :message="$t('oneterm.pam.loadFailed')" />
    <ops-table
      ref="table"
      :data="rows"
      :height="tableHeight"
      :row-config="{ keyField: 'id' }"
      :scroll-y="{ enabled: true, gt: 20 }"
      :empty-text="$t('oneterm.passwordView.noAccounts')"
      size="small"
      stripe
      resizable
      show-overflow
      show-header-overflow
      class="ops-stripe-table">
      <vxe-column field="name" :title="$t('oneterm.name')" min-width="170" />
      <vxe-column field="account" :title="$t('oneterm.account')" min-width="150" />
      <vxe-column :title="$t('oneterm.passwordView.credentialColumn')" width="125"><template #default="{ row }">{{ $t(Number(row.account_type) === 2 ? 'oneterm.credential.privateKey' : 'oneterm.password') }}</template></vxe-column>
      <vxe-column :title="$t('status')" min-width="175"><template #default="{ row }"><span :class="{ 'workstation-passwords-approved': row.view.can_view }">{{ stateLabel(row.view) }}</span></template></vxe-column>
      <vxe-column :title="$t('oneterm.passwordView.validUntil')" min-width="170"><template #default="{ row }">{{ row.view.request && row.view.request.granted_until ? formatTime(row.view.request.granted_until) : '-' }}</template></vxe-column>
      <vxe-column :title="$t('operation')" min-width="220" :fixed="windowWidth >= 900 ? 'right' : null"><template #default="{ row }">
        <CredentialReveal
          v-if="row.view.can_view || row.view.can_cancel"
          :id="row.id"
          :name="row.name"
          :auth-type="Number(row.account_type)"
          :button-label="actionLabel(row)"
          :disabled="failed || (!row.view.can_view && !row.view.can_request && !row.view.can_cancel)"
          :load="read"
          @change="updateStatus(row, $event)" />
      </template></vxe-column>
    </ops-table>
    <div class="workstation-passwords-pagination">
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
</template>

<script>
import moment from 'moment'
import { mapState } from 'vuex'
import { getPasswordViewAccounts, readPasswordView } from '@/modules/oneterm/api/pam'
import CredentialReveal from '@/modules/oneterm/components/credentialReveal.vue'
import { passwordViewStateLabel } from '@/modules/oneterm/components/passwordViewState'

export default {
  name: 'WorkstationPasswordView',
  components: { CredentialReveal },
  props: { accountId: { type: Number, default: 0 } },
  data() {
    return { rows: [], total: 0, search: '', page: 1, pageSize: 20, loading: false, failed: false, generation: 0, timer: null }
  },
  computed: {
    ...mapState({ windowHeight: state => state.windowHeight, windowWidth: state => state.windowWidth }),
    tableHeight() { return Math.max(240, this.windowHeight - 260) }
  },
  watch: { accountId: { immediate: true, handler() { this.load(1) } } },
  mounted() { document.addEventListener('visibilitychange', this.visibilityChanged) },
  beforeDestroy() {
    this.generation += 1
    clearTimeout(this.timer)
    document.removeEventListener('visibilitychange', this.visibilityChanged)
  },
  methods: {
    formatTime(value) { return moment(value).format('YYYY-MM-DD HH:mm:ss') },
    stateLabel(view) { return passwordViewStateLabel(view, this.$t.bind(this)) },
    actionLabel(row) {
      if (row.view.can_view) return this.$t(Number(row.account_type) === 2 ? 'oneterm.credential.viewPrivateKey' : 'oneterm.credential.viewPassword')
      if (row.view.can_cancel) return this.$t('oneterm.passwordView.viewRequest')
      return this.$t(Number(row.account_type) === 2 ? 'oneterm.passwordView.requestPrivateKey' : 'oneterm.passwordView.requestPassword')
    },
    read(id, token) { return readPasswordView('account', id, token).then(response => response.data) },
    updateStatus(row, status) {
      this.$set(row, 'view', status)
      if (status.can_cancel && !this.timer && !document.hidden) this.timer = setTimeout(() => this.load(this.page), 15000)
    },
    visibilityChanged() {
      clearTimeout(this.timer)
      if (!document.hidden) this.load(this.page)
    },
    async load(page = 1, pageSize = this.pageSize) {
      const generation = ++this.generation
      clearTimeout(this.timer)
      this.timer = null
      this.loading = true
      try {
        const response = await getPasswordViewAccounts({ page_index: page, page_size: pageSize, search: this.search, account_id: this.accountId || undefined })
        if (generation !== this.generation) return
        this.rows = response.data.list || []
        this.total = response.data.count || 0
        this.page = page
        this.pageSize = pageSize
        this.failed = false
      } catch (error) {
        if (generation === this.generation) this.failed = true
      } finally {
        if (generation === this.generation) {
          this.loading = false
          if (!document.hidden && this.rows.some(row => row.view.can_cancel)) this.timer = setTimeout(() => this.load(this.page), 15000)
        }
      }
    }
  }
}
</script>

<style scoped>
.workstation-passwords { padding: 8px 0; min-width: 0; }
.workstation-passwords-toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }
.workstation-passwords-toolbar .ant-input-search { width: 280px; max-width: 100%; }
.workstation-passwords-toolbar > :last-child { margin-left: auto; flex-shrink: 0; }
.workstation-passwords-approved { color: #23834d; }
.workstation-passwords-pagination { display: flex; justify-content: flex-end; padding-top: 16px; }
</style>
