<template>
  <a-drawer
    :visible="visible"
    :title="form.id ? $t('oneterm.pam.editApplication') : $t('oneterm.pam.createApplication')"
    :width="drawerWidth"
    :closable="!busy"
    :mask-closable="!busy"
    :destroy-on-close="true"
    :body-style="{ padding: 0, display: 'flex', flexDirection: 'column', height: 'calc(100% - 55px)', overflow: 'hidden' }"
    @close="close"
  >
    <a-form-model ref="form" :model="form" :rules="rules" layout="vertical" class="pam-editor">
      <a-form-model-item :label="$t('oneterm.name')" prop="name">
        <a-input v-model="form.name" :max-length="128" :disabled="busy" />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.pam.serviceIdentity')" prop="acl_uid">
        <a-select
          v-model="form.acl_uid"
          show-search
          :filter-option="false"
          :disabled="!!form.id || busy"
          :loading="subjectLoading"
          :placeholder="$t('placeholder2')"
          @search="searchSubjects"
        >
          <a-select-option v-for="subject in subjects" :key="subject.uid" :value="subject.uid" :disabled="subject.block">
            {{ subject.nickname || subject.username }} ({{ subject.username }})
          </a-select-option>
        </a-select>
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.pam.purpose')" prop="purpose" class="pam-full-width">
        <a-textarea v-model="form.purpose" :max-length="1024" :auto-size="{ minRows: 2, maxRows: 4 }" :disabled="busy" />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.pam.allowedActions')" prop="actions">
        <a-checkbox-group v-model="form.actions" :disabled="busy">
          <a-checkbox v-for="action in capabilities.application_actions || []" :key="action" :value="action">
            {{ actionLabel(action) }}
          </a-checkbox>
        </a-checkbox-group>
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.pam.applicationStatus')">
        <a-switch v-model="form.enabled" :disabled="busy" />
      </a-form-model-item>
      <a-form-model-item v-for="kind in capabilities.application_target_kinds || []" :key="kind" :label="kindLabel(kind)">
        <a-select
          v-model="form.targets[kind]"
          mode="multiple"
          show-search
          :filter-option="false"
          :disabled="busy"
          :loading="Boolean(targetLoading[kind])"
          :max-tag-count="3"
          :placeholder="$t('placeholder2')"
          @search="(value) => searchTargets(kind, value)"
        >
          <a-select-option v-for="target in targetOptions[kind] || []" :key="target.id" :value="target.id" :disabled="target.unavailable">
            {{ target.name }} ({{ target.account }})
          </a-select-option>
        </a-select>
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.pam.sourceRange')" class="pam-full-width">
        <a-checkbox v-model="restrictSource" :disabled="busy">{{ $t('oneterm.pam.limitSources') }}</a-checkbox>
        <a-textarea
          v-if="restrictSource"
          v-model="sourceText"
          :disabled="busy"
          :auto-size="{ minRows: 2, maxRows: 5 }"
          :placeholder="'192.0.2.0/24'"
          class="pam-source-input"
        />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.pam.validUntil')" class="pam-full-width">
        <div class="pam-expiry">
          <a-date-picker v-if="!permanent" v-model="expiresAt" show-time :disabled="busy" format="YYYY-MM-DD HH:mm" />
          <a-checkbox v-model="permanent" :disabled="busy">{{ $t('oneterm.pam.permanent') }}</a-checkbox>
        </div>
      </a-form-model-item>
    </a-form-model>
    <div class="pam-editor-footer">
      <a-button :disabled="busy" @click="close">{{ $t('cancel') }}</a-button>
      <a-button type="primary" :loading="busy" @click="submit"><a-icon type="save" />{{ $t('save') }}</a-button>
    </div>
  </a-drawer>
</template>

<script>
import debounce from 'lodash/debounce'
import moment from 'moment'
import { mapState } from 'vuex'
import { getPAMSubjects, getPAMTargets } from '@/modules/oneterm/api/pam'
import { applicationPayload } from './form'

export default {
  name: 'PAMApplicationEditor',
  data() {
    return {
      visible: false,
      busy: false,
      generation: 0,
      form: { name: '', acl_uid: undefined, purpose: '', actions: [], targets: {}, enabled: false, revision: 0 },
      capabilities: {},
      subjects: [],
      subjectLoading: false,
      subjectRequest: 0,
      targetOptions: {},
      targetLoading: {},
      targetGenerations: {},
      restrictSource: false,
      sourceText: '',
      permanent: false,
      expiresAt: null,
      rules: {
        name: [{ required: true, message: this.$t('placeholder1') }],
        acl_uid: [{ required: true, message: this.$t('placeholder2') }],
        purpose: [{ required: true, message: this.$t('placeholder1') }]
      }
    }
  },
  computed: {
    ...mapState({ windowWidth: (state) => state.windowWidth }),
    drawerWidth() { return Math.min(640, this.windowWidth || window.innerWidth) }
  },
  created() {
    this.searchSubjects = debounce(this.loadSubjects, 250)
    this.targetSearches = {}
  },
  beforeDestroy() {
    this.generation += 1
    this.searchSubjects.cancel()
    Object.values(this.targetSearches).forEach((search) => search.cancel())
  },
  methods: {
    actionLabel(action) {
      return action === 'retrieve' ? this.$t('oneterm.credential.permissions.retrieve') : action
    },
    kindLabel(kind) {
      const labels = { account: 'oneterm.pam.accountScope', gateway: 'oneterm.pam.gatewayScope' }
      return labels[kind] ? this.$t(labels[kind]) : kind
    },
    open(record, capabilities) {
      this.generation += 1
      this.capabilities = capabilities
      this.busy = false
      this.form = {
        id: record?.id,
        name: record?.name || '',
        acl_uid: record?.acl_uid,
        purpose: record?.purpose || '',
        actions: [...(record?.actions || [])],
        targets: {},
        enabled: record?.enabled || false,
        revision: record?.revision || 0
      }
      const kinds = new Set([...(capabilities.application_target_kinds || []), ...Object.keys(record?.targets || {})])
      kinds.forEach((kind) => { this.$set(this.form.targets, kind, [...(record?.targets?.[kind] || [])]) })
      this.sourceText = (record?.source_cidrs || []).join('\n')
      this.restrictSource = Boolean(this.sourceText)
      this.permanent = Boolean(record && !record.expires_at)
      this.expiresAt = record?.expires_at ? moment(record.expires_at) : moment().add(90, 'days')
      this.subjects = []
      this.targetOptions = {}
      this.targetGenerations = {}
      this.visible = true
      this.loadSubjects('')
      kinds.forEach((kind) => this.loadTargets(kind, ''))
      this.$nextTick(() => this.$refs.form && this.$refs.form.clearValidate())
    },
    async loadSubjects(search) {
      const generation = this.generation
      const requestId = ++this.subjectRequest
      const selectedUID = this.form.acl_uid
      this.subjectLoading = true
      try {
        const result = await getPAMSubjects({ q: search, page_size: 30 })
        let subjects = result.users || []
        if (selectedUID && !subjects.some((subject) => subject.uid === selectedUID)) {
          const selected = await getPAMSubjects({ uids: String(selectedUID), page_size: 1 })
          subjects = [...(selected.users || []), ...subjects]
          if (!subjects.some((subject) => subject.uid === selectedUID)) {
            subjects.unshift({ uid: selectedUID, nickname: this.$t('oneterm.pam.unavailableIdentity'), username: `#${selectedUID}`, block: true })
          }
        }
        if (generation === this.generation && requestId === this.subjectRequest) this.subjects = subjects
      } catch (error) {
        // The shared request handler reports lookup failures.
      } finally {
        if (generation === this.generation && requestId === this.subjectRequest) this.subjectLoading = false
      }
    },
    searchTargets(kind, value) {
      if (!this.targetSearches[kind]) this.targetSearches[kind] = debounce((query) => this.loadTargets(kind, query), 250)
      this.targetSearches[kind](value)
    },
    async loadTargets(kind, search) {
      const generation = this.generation
      const requestId = (this.targetGenerations[kind] || 0) + 1
      this.$set(this.targetGenerations, kind, requestId)
      this.$set(this.targetLoading, kind, true)
      try {
        const result = await getPAMTargets({ kind, search, page_size: 30 })
        const values = new Map((this.targetOptions[kind] || []).map((item) => [item.id, item]))
        ;(result.data?.list || []).forEach((item) => values.set(item.id, item))
        const missing = (this.form.targets[kind] || []).filter((id) => !values.has(id))
        for (let index = 0; index < missing.length; index += 200) {
          const selected = await getPAMTargets({ kind, ids: missing.slice(index, index + 200).join(','), page_size: 200 })
          ;(selected.data?.list || []).forEach((item) => values.set(item.id, item))
        }
        if (generation === this.generation && this.targetGenerations[kind] === requestId) {
          const visible = new Set([...(result.data?.list || []).map((item) => item.id), ...(this.form.targets[kind] || [])])
          ;(this.form.targets[kind] || []).forEach((id) => {
            if (!values.has(id)) values.set(id, { id, name: this.$t('oneterm.pam.unavailableTarget', { id }), account: '-', unavailable: true })
          })
          this.$set(this.targetOptions, kind, [...values.values()].filter((item) => visible.has(item.id)))
        }
      } catch (error) {
        // Keep selected values intact when a lookup temporarily fails.
      } finally {
        if (generation === this.generation && this.targetGenerations[kind] === requestId) this.$set(this.targetLoading, kind, false)
      }
    },
    close() {
      if (this.busy) return
      this.generation += 1
      this.visible = false
      this.$emit('cancel')
    },
    finish(success) {
      this.busy = false
      if (success) this.close()
    },
    submit() {
      if (this.busy) return
      this.$refs.form.validate((valid) => {
        if (!valid) return
        const targets = Object.values(this.form.targets).reduce((count, ids) => count + ids.length, 0)
        if (this.form.enabled && (!this.form.actions.length || !targets)) {
          this.$message.error(this.$t('oneterm.pam.requiredGrant'))
          return
        }
        if (!this.permanent && (!this.expiresAt || !this.expiresAt.isValid() || (this.form.enabled && this.expiresAt.isBefore(moment())))) {
          this.$message.error(this.$t('oneterm.pam.invalidExpiry'))
          return
        }
        if (this.restrictSource && !this.sourceText.trim()) {
          this.$message.error(this.$t('oneterm.pam.sourceRequired'))
          return
        }
        this.busy = true
        this.$emit('submit', { id: this.form.id, data: applicationPayload(this.form, this) })
      })
    }
  }
}
</script>

<style lang="less" scoped>
.pam-editor {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 20px 24px;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  column-gap: 16px;
  align-content: start;
}
.pam-editor .ant-form-item { min-width: 0; margin-bottom: 16px; }
.pam-editor /deep/ .ant-form-item-label { white-space: normal; }
.pam-full-width { grid-column-start: 1; grid-column-end: -1; }
.pam-editor-footer {
  flex-shrink: 0;
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 14px 24px;
  border-top: 1px solid #eeeeee;
  background: #ffffff;
}
.pam-source-input { margin-top: 8px; font-family: monospace; }
.pam-expiry { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; }
@media (max-width: 600px) {
  .pam-editor { grid-template-columns: minmax(0, 1fr); padding: 16px 20px; }
}
</style>
