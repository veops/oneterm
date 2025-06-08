<template>
  <div>
    <a-select
      :value="currentResolution"
    >
      <a-select-option
        v-for="(item) in resolutionOptions"
        :key="item.value"
        @click="clickMenuItem(item.value)"
      >
        <span class="resolution-select-text" v-if="item.value === 'custom'">
          {{ item.label }}
          <span class="resolution-select-text-tip">({{ customValue }})</span>
        </span>
        <span v-else>{{ item.label }}</span>
      </a-select-option>
    </a-select>

    <CustomModal
      ref="customModalRef"
      @ok="handleCustomModalOk"
    />
  </div>
</template>

<script>
import CustomModal from './customModal.vue'

export default {
  name: 'ResolutionSetting',
  components: {
    CustomModal
  },
  props: {
    value: {
      type: String,
      default: 'auto',
    },
  },
  model: {
    prop: 'value',
    event: 'change',
  },
  data() {
    return {
      resolutionOptions: [
        { label: this.$t('oneterm.terminalDisplay.auto'), value: 'auto' },
        { label: '1024x768', value: '1024x768' },
        { label: '1280x720', value: '1280x720' },
        { label: '1600x900', value: '1600x900' },
        { label: '1920x1080', value: '1920x1080' },
        { label: '2560x1440', value: '2560x1440' },
        { label: '3480x2160', value: '3480x2160' },
        { label: this.$t('oneterm.terminalDisplay.custom'), value: 'custom' }
      ],
      customValue: '800x600'
    }
  },
  computed: {
    currentResolution: {
      get() {
        if (!this.resolutionOptions.find((option) => option.value === this.value)) {
          return 'custom'
        }

        return this.value
      }
    },
    displayText() {
      if (this.currentResolution === 'auto') {
        return this.$t('oneterm.terminalDisplay.auto')
      }

      const findResolution = this.resolutionOptions.find((option) => option.value === this.value)
      if (findResolution) {
        return this.value
      } else {
        return `${this.$t('oneterm.terminalDisplay.custom')} ${this.customValue}`
      }
    }
  },
  watch: {
    value: {
      immediate: true,
      handler(newVal) {
        if (!this.resolutionOptions.find((option) => option.value === newVal)) {
          this.customValue = newVal
        }
      },
    }
  },
  methods: {
    clickMenuItem(value) {
      if (value === 'custom') {
        this.$emit('change', this.customValue)
        this.$refs.customModalRef.open(this.customValue)
      } else {
        this.$emit('change', value)
      }
    },

    handleCustomModalOk(value) {
      this.$emit('change', value)
    }
  },
}
</script>

<style lang="less" scoped>
.resolution-select-text {
  display: flex;
  align-items: baseline;

  &-tip {
    margin-left: 4px;
    font-size: 12px;
    color: @text-color_3;
    font-weight: 400;
  }
}
</style>
