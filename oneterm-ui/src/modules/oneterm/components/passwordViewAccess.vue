<template>
  <a-modal
    :visible="visible"
    :title="title || $t('oneterm.passwordView.requestTitle')"
    :width="720"
    :footer="null"
    :destroy-on-close="true"
    :body-style="{ maxHeight: 'calc(100vh - 160px)', overflowY: 'auto' }"
    @cancel="cancel"
  >
    <a-spin :spinning="loading">
      <a-alert v-if="failed" type="error" show-icon :message="$t('oneterm.pam.loadFailed')">
        <a-button
          slot="description"
          type="link"
          @click="refresh"
        ><a-icon type="reload" />{{ $t('oneterm.pam.retryLoad') }}</a-button
        >
      </a-alert>
      <template v-if="access">
        <div class="password-view-status" :class="'password-view-status-' + stateTone" role="status">
          <a-icon :type="stateIcon" />
          <strong>{{ stateLabel }}</strong>
        </div>
        <a-descriptions :column="1" size="small">
          <a-descriptions-item :label="$t('oneterm.account')">{{ name }}</a-descriptions-item>
          <a-descriptions-item v-if="!access.can_view" :label="$t('oneterm.passwordView.validMinutes')">{{
            $t('oneterm.pam.durationMinutes', { count: access.valid_minutes })
          }}</a-descriptions-item>
          <a-descriptions-item
            v-if="access.request && access.request.granted_until"
            :label="$t('oneterm.passwordView.validUntil')"
          >{{ formatTime(access.request.granted_until) }}</a-descriptions-item
          >
        </a-descriptions>
        <a-alert
          v-if="!access.can_view"
          type="warning"
          show-icon
          :message="$t('oneterm.passwordView.approvalUnavailable')"
        />
        <div class="password-view-actions">
          <a-tooltip
            :title="$t('oneterm.passwordView.refreshStatus')"
          ><a-button
            :aria-label="$t('oneterm.passwordView.refreshStatus')"
            :loading="refreshing"
            :disabled="submitting"
            @click="refresh"
          ><a-icon type="reload" /></a-button
          ></a-tooltip>
          <a-button
            v-if="access.can_cancel"
            :disabled="submitting || failed || refreshing"
            @click="withdraw"
          ><a-icon type="stop" />{{ withdrawLabel }}</a-button
          >
          <a-button
            v-if="access.can_view"
            type="primary"
            :disabled="submitting || failed || refreshing"
            @click="view"
          ><a-icon type="eye" />{{ title || $t('oneterm.credential.viewLoginInfo') }}</a-button
          >
          <a-button
            v-else-if="access.can_request && hasITSM"
            type="primary"
            :loading="submitting"
            :disabled="!template || !reason.trim() || failed || refreshing"
            @click="submit"
          ><a-icon type="send" />{{ $t('oneterm.passwordView.submitApproval') }}</a-button
          >
        </div>
      </template>
    </a-spin>
  </a-modal>
</template>

<script>
import moment from 'moment'
import { getPasswordViewStatus, cancelPAMRequest } from '@/modules/oneterm/api/pam'
import { passwordViewStateLabel } from './passwordViewState'

export default {
  name: 'PasswordViewAccess',
  props: {
    title: { type: String, default: '' },
    id: { type: Number, required: true },
    kind: { type: String, default: 'account' },
    name: { type: String, default: '' },
  },
  data() {
    return {
      visible: false,
      loading: false,
      refreshing: false,
      failed: false,
      access: null,
      submitting: false,
      generation: 0,
      action: 0,
      timer: null,
    }
  },
  computed: {
    stateLabel() {
      return passwordViewStateLabel(this.access, this.$t.bind(this))
    },
    stateTone() {
      if (this.access?.can_view) return 'success'
      if (this.access?.state === 'rejected') return 'error'
      return 'neutral'
    },
    stateIcon() {
      return this.stateTone === 'success'
        ? 'check-circle'
        : this.stateTone === 'error'
        ? 'close-circle'
        : 'clock-circle'
    },
    withdrawLabel() {
      return this.$t(this.access?.can_view ? 'oneterm.passwordView.endViewing' : 'oneterm.pam.withdrawRequest')
    },
  },
  mounted() {
    document.addEventListener('visibilitychange', this.visibilityChanged)
  },
  beforeDestroy() {
    this.close()
    document.removeEventListener('visibilitychange', this.visibilityChanged)
  },
  methods: {
    formatTime(value) {
      return moment(value).format('YYYY-MM-DD HH:mm:ss')
    },
    async open(action) {
      this.close()
      this.action = action
      this.visible = true
      const ready = await this.refresh()
      if (ready) this.close()
      return ready
    },
    close() {
      this.visible = false
      this.generation += 1
      clearTimeout(this.timer)
      this.timer = null
      this.access = null
      this.loading = false
      this.refreshing = false
      this.submitting = false
    },
    cancel() {
      this.close()
      this.$emit('cancel')
    },
    visibilityChanged() {
      if (document.hidden) {
        clearTimeout(this.timer)
        this.timer = null
      } else if (this.visible) this.refresh()
    },
    async refresh() {
      if (!this.visible || this.refreshing) return false
      const generation = this.generation
      clearTimeout(this.timer)
      this.refreshing = true
      this.loading = !this.access
      this.failed = false
      try {
        const response = await getPasswordViewStatus(this.kind, this.id)
        if (generation !== this.generation || !this.visible) return false
        this.access = response.data
        this.$emit('change', this.access)
        if (this.access.can_cancel && !document.hidden) this.timer = setTimeout(() => this.refresh(), 15000)
        return this.access.can_view
      } catch (error) {
        if (generation === this.generation) {
          this.failed = true
          if ([401, 403, 404].includes(error?.response?.status)) this.access = null
          this.$message.error(error?.response?.data?.message || this.$t('oneterm.pam.loadFailed'))
        }
        return false
      } finally {
        if (generation === this.generation) {
          this.loading = false
          this.refreshing = false
        }
      }
    },
    withdraw() {
      const request = this.access?.request
      if (!request || !this.access.can_cancel || this.submitting || this.failed || this.refreshing) return
      const generation = this.generation
      this.$confirm({
        title: this.withdrawLabel,
        content: this.$t(
          this.access.can_view ? 'oneterm.passwordView.endViewingConfirm' : 'oneterm.passwordView.withdrawConfirm'
        ),
        onOk: async () => {
          if (generation !== this.generation || !this.visible) return
          this.submitting = true
          try {
            await cancelPAMRequest(request.id, { revision: request.revision })
            if (generation === this.generation) await this.refresh()
          } catch (error) {
            if (generation === this.generation) {
              this.$message.error(error?.response?.data?.message || this.$t('requestError'))
            }
          } finally {
            if (generation === this.generation) this.submitting = false
          }
        },
      })
    },
    view() {
      if (!this.access?.can_view || this.failed || this.refreshing || this.submitting) return
      const action = this.action
      this.close()
      this.$emit('ready', { action })
    },
  },
}
</script>

<style scoped>
.password-view-status {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 20px;
  font-size: 14px;
  color: #4e5969;
}
.password-view-status strong {
  font-weight: 500;
  overflow-wrap: anywhere;
}
.password-view-status-success {
  color: #23834d;
}
.password-view-status-error {
  color: #c63838;
}
.password-view-reason {
  margin-top: 16px;
}
.password-view-actions {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 20px;
}
</style>
