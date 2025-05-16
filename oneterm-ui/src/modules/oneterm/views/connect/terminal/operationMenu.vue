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
        <a-menu-item @click="toggleFullScreen" >
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
          @click="openCommandDrawer"
        >
          <ops-icon type="oneterm-commandrecord" />
          {{ $t('oneterm.quickCommand.name') }}
        </a-menu-item>

        <a-menu-item
          @click="openSystemSetting('displaySetting')"
        >
          <ops-icon type="veops-setting" />
          {{ $t('oneterm.terminalDisplay.displaySetting') }}
        </a-menu-item>

        <a-menu-item
          @click="openSystemSetting('themeSetting')"
        >
          <ops-icon type="ops-setting-theme" />
          {{ $t('oneterm.terminalDisplay.themeSetting') }}
        </a-menu-item>
      </a-menu>
    </a-dropdown>

    <CommandDrawer
      ref="commandDrawerRef"
      @write="writeCommand"
    />
  </div>
</template>

<script>
import CommandDrawer from '@/modules/oneterm/views/systemSettings/quickCommand/commandDrawer.vue'

export default {
  name: 'OperationMenu',
  components: {
    CommandDrawer
  },
  props: {
    openFullScreen: {
      type: Boolean,
      default: false
    }
  },
  methods: {
    toggleFullScreen() {
      this.$emit('toggleFullScreen')
    },

    openCommandDrawer() {
      this.$refs.commandDrawerRef.open()
    },

    writeCommand(content) {
      this.$emit('toggleFullScreen', content)
    },

    openSystemSetting(type) {
      this.$emit('openSystemSetting', type)
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
