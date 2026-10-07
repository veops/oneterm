<template>
  <div
    class="code-input"
    @paste.stop="handlePaste"
  >
    <a-input-number
      v-for="(item, index) in codeList"
      :key="index"
      class="code-input-item"
      ref="codeInputRef"
      :max="100"
      :min="0"
      :precision="0"
      :value="item"
      :formatter="handleFormatter"
      @change="(value) => handleCodeChange(value, index)"
      @keydown="(e) => handleCodeKeydown(e, index)"
      @paste.native.capture.prevent="() => false"
    />
  </div>
</template>

<script>
export default {
  name: 'CodeInputBox',
  model: {
    prop: 'value',
    event: 'change',
  },
  props: {
    value: {
      type: Array,
      default: () => [],
    }
  },
  computed: {
    codeList: {
      get() {
        return this.value
      },
      set(newValue) {
        this.$emit('change', newValue)
      }
    }
  },
  methods: {
    handleCodeKeydown(e, index) {
      const inputList = this.$refs?.codeInputRef
      const codeList = [...this.value]

      if (inputList?.length) {
        switch (e.key) {
          case 'Backspace':
            if (index > 0) {
              setTimeout(() => {
                inputList[index - 1].focus()
              })
            }
            break
          case 'Delete':
            codeList.splice(index, 1, '')
            this.$emit('change', codeList)
            break
          case 'Home':
            inputList[0].focus()
            break
          case 'End':
            inputList[inputList.length - 1].focus()
            break
          case 'ArrowLeft':
            if (index > 0) {
              inputList[index - 1].focus()
            }
            break
          case 'ArrowRight':
            if (index < inputList.length - 1) {
              inputList[index + 1].focus()
            }
            break
          case 'Enter':
            this.$emit('enter')
            break
          default:
            break
        }
      }
    },

    handlePaste(e) {
      e.stopPropagation()
      e.clipboardData.items[0].getAsString((str) => {
        if (str.toString().length === this.value.length) {
          this.$emit('change', str.split(''))
        } else {
          this.$emit('change', this.value.map(() => ''))
        }
      })
    },

    handleCodeChange(value, index) {
      const codeList = [...this.value]
      const newValue = this.handleFormatter(value)
      codeList[index] = newValue
      this.$emit('change', codeList)
      if (newValue) {
        const inputList = this.$refs?.codeInputRef
        if (index < inputList.length - 1) {
          inputList[index + 1].focus()
        }
      }
    },

    handleFormatter(value) {
      const strValue = String(value).trim()
      const sliceValue = strValue.slice(strValue.length - 1)
      return Number.isInteger(Number(sliceValue)) ? sliceValue : ''
    }
  }
}
</script>

<style lang="less" scoped>
.code-input {
  display: flex;
  align-items: center;
  justify-content: center;
  column-gap: 11px;

  &-item {
    width: 40px;
    height: 50px;
    line-height: 48px;
    font-size: 22px;
    border-radius: 2px;
    flex-shrink: 0;
    border: 1px solid @border-color-base;
    box-shadow: 0px 1px 2px 0px rgba(160, 173, 198, 0.25);

    &:hover {
      border: 1px solid @primary-color;
    }
  }

  /deep/ .ant-input-number {
    text-align: center;

    &-handler-wrap {
      display: none;
    }

    &-input {
      text-align: center;
    }

    &-focused {
      border-radius: 2px;
      border: 1px solid @primary-color;
      background-color: @primary-color_5;
    }
  }
}
</style>
