<template>
  <a-modal
    :visible="visible"
    :title="$t('oneterm.guacamole.clipboard')"
    width="500px"
    :okText="$t('copy')"
    @cancel="handleCancel"
    @ok="handleOk"
  >
    <a-textarea
      id="guacamole-clipboard-textarea"
      v-model="clipboardContent"
      :placeholder="$t('placeholder1')"
      :auto-size="{ minRows: 5, maxRows: 8 }"
      ref="clipboardTextareaRef"
    />
  </a-modal>
</template>

<script>
export default {
  name: 'ClipboardModal',
  data() {
    return {
      visible: false,
      clipboardContent: ''
    }
  },
  methods: {
    open() {
      this.visible = true

      if (navigator.clipboard) {
        navigator.clipboard.readText().then((text) => {
          if (text) {
            this.clipboardContent = text
          }
        })
      }
    },
    handleCancel() {
      this.clipboardContent = ''
      this.visible = false
    },
    handleOk() {
      this.$emit('ok', this.clipboardContent)
      this.handleCancel()
    }
  }
}
</script>

<style lang="less" scoped>

</style>
