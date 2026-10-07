<template>
  <div>
    <a-form-model-item v-if="editing" key="replace-credential" :label="$t('oneterm.credential.replaceStored')">
      <a-switch :checked="replace" @change="setReplace" />
    </a-form-model-item>
    <a-form-model-item key="account-type" :label="$t('oneterm.credential.authMethod')" prop="account_type">
      <a-radio-group :value="value.account_type" :disabled="editing && !replace" @change="setType">
        <a-radio :value="1">{{ $t('oneterm.password') }}</a-radio>
        <a-radio :value="2">{{ $t('oneterm.credential.privateKey') }}</a-radio>
      </a-radio-group>
    </a-form-model-item>
    <template v-if="!editing || replace">
      <a-form-model-item
        v-if="value.account_type === 1"
        key="password"
        :label="$t('oneterm.password')"
        prop="password"
        :rules="[{ required: requirePassword, message: $t('placeholder1') }]"
      >
        <a-input-password :value="value.password" autocomplete="new-password" @input="setField('password', $event)" />
      </a-form-model-item>
      <template v-else>
        <a-form-model-item
          :label="$t('oneterm.credential.privateKey')"
          key="private-key"
          prop="pk"
          :rules="[{ required: true, message: $t('placeholder1') }]"
        >
          <a-textarea
            :value="value.pk"
            :auto-size="{ minRows: 4, maxRows: 8 }"
            class="credential-key"
            autocomplete="off"
            spellcheck="false"
            @input="setField('pk', $event)"
          />
        </a-form-model-item>
        <a-form-model-item key="passphrase" :label="$t('oneterm.phrase')" prop="phrase">
          <a-input-password :value="value.phrase" autocomplete="new-password" @input="setField('phrase', $event)" />
        </a-form-model-item>
      </template>
    </template>
  </div>
</template>

<script>
export default {
  name: 'CredentialEditor',
  props: {
    value: { type: Object, required: true },
    editing: { type: Boolean, default: false },
    replace: { type: Boolean, default: false },
    originalType: { type: Number, default: 1 },
    requirePassword: { type: Boolean, default: true }
  },
  methods: {
    setField(field, event) {
      this.$emit('input', { ...this.value, [field]: event.target.value })
    },
    setType(event) {
      this.$emit('input', { ...this.value, account_type: event.target.value, password: '', pk: '', phrase: '' })
    },
    setReplace(replace) {
      this.$emit('update:replace', replace)
      this.$emit('input', { ...this.value, account_type: this.originalType, password: '', pk: '', phrase: '' })
    }
  }
}
</script>

<style scoped>
.credential-key {
  font-family: monospace;
  font-size: 12px;
}
</style>
