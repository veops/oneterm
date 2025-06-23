<template>
  <vxe-table v-bind="$attrs" v-on="new$listeners" ref="xTable">
    <slot></slot>
    <template #empty>
      <slot name="empty">
        <div :style="{ paddingTop: '10px' }">
          <img :style="{ width: '140px', height: '120px' }" :src="require('@/assets/data_empty.png')" />
          <div>{{ $t('noData') }}</div>
        </div>
      </slot>
    </template>
    <template #loading>
      <slot name="loading"></slot>
    </template>
  </vxe-table>
</template>

<script>
import _ from 'lodash'
/**
 * This component is used in the same way as vxe-table, but when calling its methods, you need to call `getVxetableRef()` to get the vxe-table component first.
 */
export default {
  name: 'OpsTable',
  data() {
    return {
      // isShifting: false,
      // lastIndex: -1,
      lastSelected: [],
      currentSelected: [],
    }
  },
  computed: {
    new$listeners() {
      if (!Object.keys(this.$listeners).length) {
        return this.$listeners
      }
      return Object.assign(this.$listeners, {
        // overriding vxe-table change events
        // 'checkbox-change': this.selectChangeEvent,
        'checkbox-range-change': this.checkboxRangeChange,
        'checkbox-range-start': this.checkboxRangeStart,
        'checkbox-range-end': this.checkboxRangeEnd,
      })
    },
  },
  mounted() {
    // window.onkeydown = (e) => {
    //   if (e.key === 'Shift') {
    //     this.isShifting = true
    //   }
    // }
    // window.onkeyup = (e) => {
    //   if (e.key === 'Shift') {
    //     this.isShifting = false
    //     this.lastIndex = -1
    //   }
    // }
  },
  beforeDestroy() {
    // window.onkeydown = ''
    // window.onkeyup = ''
  },
  methods: {
    getVxetableRef() {
      return this.$refs.xTable
    },
    // selectChangeEvent(e) {
    //   const xTable = this.$refs.xTable
    //   const { lastIndex } = this
    //   const currentIndex = e.rowIndex
    //   const { tableData } = xTable.getTableData()
    //   if (lastIndex > -1 && this.isShifting) {
    //     let start = lastIndex
    //     let end = currentIndex
    //     if (lastIndex > currentIndex) {
    //       start = currentIndex
    //       end = lastIndex
    //     }
    //     const rangeData = tableData.slice(start, end + 1)
    //     xTable.setCheckboxRow(rangeData, true)
    //   }
    //   this.lastIndex = currentIndex
    //   this.$emit('checkbox-change', { ...e, records: xTable.getCheckboxRecords() })
    // },
    checkboxRangeStart(e) {
      const xTable = this.$refs.xTable
      const lastSelected = xTable.getCheckboxRecords()
      const selectedReserve = xTable.getCheckboxReserveRecords()
      this.lastSelected = [...lastSelected, ...selectedReserve]
      this.$emit('checkbox-range-start', e)
    },
    checkboxRangeChange(e) {
      const xTable = this.$refs.xTable
      xTable.setCheckboxRow(this.lastSelected, true)
      this.currentSelected = e.records
      // this.lastSelected = [...new Set([...this.lastSelected, ...e.records])]
      this.$emit('checkbox-range-change', {
        ...e,
        records: [...xTable.getCheckboxRecords(), ...xTable.getCheckboxReserveRecords()],
      })
    },
    checkboxRangeEnd(e) {
      const xTable = this.$refs.xTable
      const isAllSelected = this.currentSelected.every((item) => {
        const _idx = this.lastSelected.findIndex((ele) => _.isEqual(ele, item))
        return _idx > -1
      })
      if (isAllSelected) {
        xTable.setCheckboxRow(this.currentSelected, false)
      }
      this.currentSelected = []
      this.lastSelected = []
      this.$emit('checkbox-range-end', {
        ...e,
        records: [...xTable.getCheckboxRecords(), ...xTable.getCheckboxReserveRecords()],
      })
    },
  },
}
</script>

<style lang="less"></style>
