<template>
  <div>
    <a-dropdown
      placement="topRight"
      class="operation-menu"
    >
      <div class="operation-menu-btn">
        <a-icon type="menu" />
      </div>

      <a-menu slot="overlay">
        <a-menu-item
          v-if="showFullScreenBtn"
          @click="toggleFullScreen"
        >
          <template v-if="openFullScreen">
            <a-icon type="fullscreen-exit" />
            {{ $t('oneterm.workStation.exitFullScreen') }}
          </template>
          <template v-else>
            <a-icon type="fullscreen" />
            {{ $t('oneterm.workStation.fullScreen') }}
          </template>
        </a-menu-item>

        <a-menu-item
          v-if="showClipboardBtn"
          @click="openClipboardModal"
        >
          <a-icon type="copy" />
          {{ $t('oneterm.guacamole.clipboard') }}
        </a-menu-item>
      </a-menu>
    </a-dropdown>

    <ClipboardModal
      ref="clipboardModalRef"
      @ok="handleClipboardOk"
    />
  </div>
</template>

<script>
import ClipboardModal from './clipboardModal.vue'

export default {
  name: 'OperationMenu',
  components: {
    ClipboardModal
  },
  props: {
    openFullScreen: {
      type: Boolean,
      default: false
    },
    showClipboardBtn: {
      type: Boolean,
      default: false
    },
    showFullScreenBtn: {
      type: Boolean,
      default: false
    }
  },
  methods: {
    toggleFullScreen() {
      this.$emit('toggleFullScreen')
    },

    openClipboardModal() {
      this.$refs.clipboardModalRef.open()
    },

    handleClipboardOk(content) {
      this.$emit('handleClipboardOk', content)
    }
  }
}
</script>

<style lang="less" scoepd>
.operation-menu {
  position: absolute;
  bottom: 20px;
  right: 30px;
  z-index: 1001;

  &-btn {
    background-color: #FFFFFF;
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 2px;
    cursor: pointer;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);

    &:hover {
      color: @text-color_5;
    }
  }
}
</style>
