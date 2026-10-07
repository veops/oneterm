<template>
  <a-form
    v-if="form"
    layout="vertical"
    class="pam-policy-shell"
  ><fieldset :disabled="saving" class="pam-policy-form">
     <a-alert
       v-if="approvalEnabled"
       type="warning"
       show-icon
       :message="$t('oneterm.passwordView.approvalUnavailable')"
     />
     <a-form-item
       :label="$t('oneterm.pam.maxRequestDuration')"
     ><a-input-number
       v-model="form.max_duration_minutes"
       :min="1"
       :max="10080"
       :precision="0"
       :disabled="disabled"
     /><span class="pam-policy-unit">{{ $t('oneterm.pam.minutes') }}</span></a-form-item
     >
     <a-form-item
       :label="$t('oneterm.pam.requestWaitingLimit')"
     ><a-input-number
       v-model="form.request_expiry_minutes"
       :min="1"
       :max="43200"
       :precision="0"
       :disabled="disabled"
     /><span class="pam-policy-unit">{{ $t('oneterm.pam.minutes') }}</span></a-form-item
     >
     <a-form-item :label="$t('oneterm.accessControl.permissionConfig')">
       <a-checkbox
         v-for="action in connectionActions"
         :key="action"
         v-model="form.connection_permissions[action]"
         :disabled="disabled"
       >{{ $t(permissionNames[action]) }}</a-checkbox
       >
     </a-form-item>
     <a-form-item
       :label="$t('oneterm.pam.allowApplications')"
     ><a-switch
       v-model="form.allow_applications"
       :disabled="disabled"
     /></a-form-item>
     <template v-if="capabilities.session_limits_enabled">
       <a-form-item
         :label="$t('oneterm.pam.maxSessions')"
       ><a-space
       ><a-switch
         :checked="form.max_sessions > 0"
         :disabled="disabled"
         @change="toggleLimit('max_sessions', $event, 1)" /><a-input-number
           v-if="form.max_sessions > 0"
           v-model="form.max_sessions"
           :min="1"
           :max="1000"
           :precision="0"
           :disabled="disabled" /></a-space
           ></a-form-item>
       <a-form-item
         :label="$t('oneterm.pam.maxSessionTime')"
       ><a-space
       ><a-switch
         :checked="form.max_session_minutes > 0"
         :disabled="disabled"
         @change="toggleLimit('max_session_minutes', $event, 60)"
       /><a-input-number
         v-if="form.max_session_minutes > 0"
         v-model="form.max_session_minutes"
         :min="1"
         :max="10080"
         :precision="0"
         :disabled="disabled"
       /><span v-if="form.max_session_minutes > 0" class="pam-policy-unit">{{
         $t('oneterm.pam.minutes')
       }}</span></a-space
       ></a-form-item
       >
     </template>
     <a-form-item
       :label="$t('oneterm.pam.sourceRange')"
     ><a-textarea
       v-model="sourceText"
       :rows="3"
       :disabled="disabled"
       :placeholder="$t('oneterm.pam.anySource')"
     /></a-form-item>
   </fieldset>
    <div class="pam-policy-footer">
      <a-button
        v-if="!disabled"
        type="primary"
        :loading="saving"
        @click="save"
      ><a-icon type="save" />{{ $t('save') }}</a-button
      >
    </div></a-form
  >
</template>

<script>
import { PERMISSION_TYPE_NAME } from '@/modules/oneterm/views/systemSettings/accessControl/constants'
import { policyPayload } from './managedAccountForm'

export default {
  name: 'PAMPolicyEditor',
  props: {
    policy: { type: Object, required: true },
    capabilities: { type: Object, required: true },
    disabled: Boolean,
    saving: Boolean,
  },
  data() {
    return {
      form: null,
      sourceText: '',
      permissionNames: PERMISSION_TYPE_NAME,
      connectionActions: ['file_upload', 'file_download', 'copy', 'paste'],
    }
  },
  computed: {
    approvalEnabled() {
      return Boolean(this.form?.allow_requests || this.form?.require_approval_actions?.length)
    },
  },
  watch: {
    policy: {
      immediate: true,
      handler(value) {
        this.form = JSON.parse(JSON.stringify(value))
        this.sourceText = (value.source_cidrs || []).join('\n')
      },
    },
  },
  methods: {
    toggleLimit(field, enabled, initial) {
      this.form[field] = enabled ? initial : 0
    },
    save() {
      if (this.approvalEnabled) {
        this.$message.error(this.$t('oneterm.passwordView.approvalUnavailable'))
        return
      }
      this.$emit('save', policyPayload(this.form, this.sourceText))
    },
  },
}
</script>

<style scoped>
.pam-policy-shell {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}
.pam-policy-form {
  flex: 1;
  min-height: 0;
  overflow: auto;
  display: grid;
  align-content: start;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  column-gap: 24px;
  border: 0;
  padding: 0;
  margin: 0;
  min-width: 0;
}
.pam-policy-unit {
  margin-left: 8px;
  color: #78818b;
}
.pam-policy-footer {
  flex-shrink: 0;
  padding: 12px 0 0;
  background: #fff;
  border-top: 1px solid #e8ebee;
}
@media (max-width: 640px) {
  .pam-policy-form {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
