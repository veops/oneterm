<template>
  <div class="password-field">
    <a-input
      v-if="showPassword"
      ref="passwordInputRef"
      :value="value"
      @blur="handleInputBlur"
      @change="handleInputChange"
    />
    <span
      v-else
    >
      {{ passwordHiddenText }}
    </span>
    <a-icon
      :type="showPassword ? 'eye-invisible' : 'eye'"
      class="password-field-icon"
      @click="toggleShowPassword"
    />
  </div>
</template>

<script>
export default {
  name: 'PasswordField',
  props: {
    value: {
      type: String,
      default: '',
    },
  },
  data() {
    return {
      showPassword: false,
      passwordHiddenText: '******'
    }
  },
  model: {
    prop: 'value',
    event: 'change',
  },
  methods: {
    handleInputChange(e) {
      this.$emit('change', e.target.value)
    },
    handleInputBlur() {
      this.showPassword = false
    },
    toggleShowPassword() {
      this.showPassword = !this.showPassword
      if (this.showPassword) {
        this.$nextTick(() => {
          if (this.$refs?.passwordInputRef?.focus) {
            this.$refs.passwordInputRef.focus()
          }
        })
      }
    }
  }
}
</script>

<style lang="less" scoped>
.password-field {
  display: flex;
  align-items: center;

  &-icon {
    margin-left: 6px;
    cursor: pointer;
    color: @primary-color_9;
  }
}
</style>
