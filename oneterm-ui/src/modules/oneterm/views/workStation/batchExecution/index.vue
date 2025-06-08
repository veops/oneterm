<template>
  <div class="batch-execution">
    <a-input
      v-model="commandValue"
      :placeholder="$t('oneterm.workStation.batchExecutionPlaceholder')"
      @pressEnter="batchWriteCommand"
    >
      <a-tooltip slot="addonAfter" :title="$t('oneterm.quickCommand.name')">
        <ops-icon
          type="quick_commands"
          class="batch-execution-input-icon"
          @click="openCommandDrawer"
        />
      </a-tooltip>
    </a-input>

    <div class="batch-execution-grid">
      <div
        v-for="(terminal, terminalIndex) in terminalList"
        :key="terminalIndex"
        class="batch-execution-grid-item"
      >
        <div class="item-header">
          <span :class="['item-header-status', !terminal.socketStatus ? 'item-header-status_error' : '']"></span>

          <a-tooltip :title="terminal.name">
            <span class="item-header-title">{{ terminal.name }}</span>
          </a-tooltip>
        </div>

        <div class="item-container">
          <TerminalPanel
            ref="terminalPanelRef"
            :assetId="terminal.assetId"
            :accountId="terminal.accountId"
            :protocol="terminal.protocol"
            :isFullScreen="false"
            :preferenceSetting="preferenceSetting"
            @open="getOfUserStat"
            @close="handleTerminalClose(terminalIndex)"
          />
        </div>
      </div>
    </div>

    <CommandDrawer
      ref="commandDrawerRef"
      @write="writeQuickCommand"
    />
  </div>
</template>

<script>
import _ from 'lodash'

import TerminalPanel from '@/modules/oneterm/views/connect/terminal/index.vue'
import CommandDrawer from '@/modules/oneterm/views/systemSettings/quickCommand/commandDrawer.vue'

export default {
  name: 'BatchExecution',
  components: {
    TerminalPanel,
    CommandDrawer
  },
  props: {
    batchExecutionData: {
      type: Array,
      default: () => []
    },
    preferenceSetting: {
      type: [Object, null],
      default: null
    }
  },
  data() {
    return {
      commandValue: '',
      terminalList: []
    }
  },
  async mounted() {
    this.terminalList = this.batchExecutionData.map((item) => {
      return {
        protocol: item.protocol,
        assetId: item.assetId,
        accountId: item.accountId,
        name: item.title,
        socketStatus: true
      }
    })
  },
  methods: {
    openCommandDrawer() {
      this.$refs.commandDrawerRef.open()
    },
    writeQuickCommand(content) {
      if (content) {
        this.commandValue = content
      }
    },
    batchWriteCommand() {
      if (
        this?.$refs?.terminalPanelRef?.length &&
        this.commandValue
      ) {
        this.$refs.terminalPanelRef.map((terminalPanel, terminalPanelIndex) => {
          if (this.terminalList?.[terminalPanelIndex]?.socketStatus) {
            terminalPanel.writeCommand(this.commandValue)
            setTimeout(() => {
              terminalPanel.writeCommand('\r')
            })
          }
        })

        this.commandValue = ''
      }
    },

    handleTerminalClose(terminalIndex) {
      this.terminalList[terminalIndex].socketStatus = false
      this.getOfUserStat(1000)
    },

    getOfUserStat: _.debounce(
      function() {
        this.$emit('getOfUserStat')
      },
      1000
    )
  }
}
</script>

<style lang="less" scoped>
.batch-execution {
  width: 100%;
  height: 100%;

  &-input-icon {
    cursor: pointer;
    font-size: 16px;
  }

  &-grid {
    margin-top: 16px;
    display: flex;
    flex-wrap: wrap;
    column-gap: 18px;
    row-gap: 16px;
    overflow-x: hidden;
    overflow-y: auto;
    height: calc(100vh - 215px);

    &-item {
      width: calc((100% - 18px) / 2);
      flex-shrink: 0;
      height: 70%;
      border-radius: 6px;
      overflow: hidden;

      .item-header {
        display: flex;
        align-items: center;
        width: 100%;
        padding: 8px 12px;
        height: 37px;
        background-color: @primary-color_7;

        &-status {
          width: 12px;
          height: 12px;
          border-radius: 50%;
          background-color: #00B42A22;
          position: relative;
          flex-shrink: 0;

          &::before {
            content: '';
            position: absolute;
            top: 50%;
            left: 50%;
            width: 6px;
            height: 6px;
            border-radius: 50%;
            margin-top: -3px;
            margin-left: -3px;
            background-color: #00B42A;
          }

          &_error {
            background-color: #F2637B22;

            &::before {
              background-color: #F2637B;
            }
          }
        }

        &-title {
          max-width: 100%;
          overflow: hidden;
          text-overflow: ellipsis;
          text-wrap: nowrap;
          margin-left: 3px;
        }
      }

      .item-container {
        width: 100%;
        height: calc(100% - 37px);
      }
    }
  }
}
</style>
